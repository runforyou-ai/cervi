"""提供原件解析、分段持久化和固定批次的分页阅读接口。"""

import json
import logging
import os
from bisect import bisect_right
from pathlib import Path
from tempfile import TemporaryDirectory
from uuid import UUID, uuid5

import psycopg
from fastapi import APIRouter, File, Form, HTTPException, UploadFile
from haystack import Document
from haystack.components.converters import MultiFileConverter, XLSXToDocument
from haystack.components.preprocessors import RecursiveDocumentSplitter
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb
from pydantic import BaseModel, Field
from pypdf import PdfReader

router = APIRouter(prefix="/knowledge")
logger = logging.getLogger(__name__)


class ProcessInput(BaseModel):
    """固定当前任务的业务归属和字符分段参数。"""

    organizationId: UUID
    knowledgeBaseId: UUID
    documentId: UUID
    processingId: UUID
    chunkLength: int = Field(ge=256, le=2048)
    chunkOverlap: int = Field(ge=0, le=200)


class SegmentInput(BaseModel):
    """定义固定批次中的分页或锚点定位。"""

    organizationId: UUID
    knowledgeBaseId: UUID
    documentId: UUID
    segmentBatchId: UUID
    anchorSegmentId: UUID | None = None
    page: int = Field(default=0, ge=0)
    pageSize: int = Field(default=20, ge=1, le=100)


# 更新阶段只作用于仍在运行的同一任务。
def set_stage(input: ProcessInput, stage: str) -> None:
    with psycopg.connect(os.environ["PG_CONN_STR"]) as connection:
        connection.execute(
            "UPDATE knowledge_documents SET status=%s, updated_at=now() "
            "WHERE id=%s AND processing_id=%s AND status NOT IN ('initial','succeeded','failed','cancelled')",
            (stage, input.documentId, input.processingId),
        )


# 将格式转换为有真实来源位置的正文，不接受空提取结果。
def convert_file(path: Path) -> list[Document]:
    extension = path.suffix.lower()
    if extension == ".json":
        text = path.read_text(encoding="utf-8-sig")
        json.loads(text)
        return [Document(content=text)]
    if extension == ".pdf":
        pdf = PdfReader(path)
        if pdf.is_encrypted:
            raise HTTPException(422, detail={"code": "encrypted_file", "stage": "extracting"})
        # 有图像却没有文字的页面需要 OCR，不能把部分提取当成整份文档成功。
        for page in pdf.pages:
            if not (page.extract_text() or "").strip() and len(page.images):
                raise HTTPException(422, detail={"code": "recognition_required", "stage": "recognizing"})
    if extension == ".xlsx":
        # 文本单元格按原值读取，避免编号前导零被推断为数字。
        return XLSXToDocument(read_excel_kwargs={"dtype": str, "keep_default_na": False}).run(sources=[path])["documents"]
    converted = MultiFileConverter().run(sources=[path])
    if converted.get("unclassified") or converted.get("failed"):
        raise HTTPException(422, detail={"code": "unsupported_file", "stage": "converting"})
    documents = converted["documents"]
    if extension == ".pdf":
        # PDF converter 用换页符连接真实页面，拆开后保留页码。
        documents = [
            Document(content=text, meta={"page_number": number})
            for doc in documents
            for number, text in enumerate((doc.content or "").split("\f"), 1)
        ]
    return documents


# 使用 Haystack 按字符切分，并只保留 converter 提供的来源信息。
def split_documents(documents: list[Document], length: int, overlap: int) -> list[dict]:
    splitter = RecursiveDocumentSplitter(
        split_length=length, split_overlap=0, split_unit="char",
        separators=["\n\n", "\n", "。", "！", "？", "；", " "],
    )
    segments = []
    for document in documents:
        text = (document.content or "").replace("\r\n", "\n").replace("\r", "\n")
        if not text.strip():
            continue
        chunks = splitter.run([Document(content=text)])["documents"]
        boundaries = []
        end = 0
        for chunk in chunks:
            end += len(chunk.content or "")
            boundaries.append(end)
        start = 0
        while start < len(text):
            end = min(start + length, len(text))
            if end < len(text):
                boundary = boundaries[bisect_right(boundaries, end) - 1]
                # 段落边界至少覆盖半段，并确保扣除重叠后仍能前进。
                if boundary >= start + max(overlap + 1, length // 2):
                    end = boundary
            # 只从原文截取连续区间，递归层级不会重复叠加重叠内容。
            content = text[start:end]
            segments.append({
                "position": len(segments) + 1,
                "content": content,
                "character_count": len(content),
                "page_number": document.meta.get("page_number"),
                "source_label": str(document.meta.get("xlsx", {}).get("sheet_name", "")),
            })
            if end == len(text):
                break
            start = end - overlap
    if not segments:
        raise HTTPException(422, detail={"code": "empty_content", "stage": "splitting"})
    return segments


# 在文档行锁内提交全部分段，删除和过期任务均不能重新写入。
def save_segments(input: ProcessInput, segments: list[dict]) -> dict:
    with psycopg.connect(os.environ["PG_CONN_STR"]) as connection:
        current = connection.execute(
            "SELECT kd.processing_id, kd.status FROM knowledge_documents kd "
            "JOIN knowledge_bases kb ON kb.id=kd.knowledge_base_id "
            "WHERE kd.id=%s AND kb.id=%s AND kb.organization_id=%s FOR UPDATE OF kd",
            (input.documentId, input.knowledgeBaseId, input.organizationId),
        ).fetchone()
        if not current or str(current[0]) != str(input.processingId) or current[1] in ("initial", "succeeded", "failed", "cancelled"):
            return {"segmentCount": 0, "stale": True}
        connection.execute(
            "DELETE FROM public.knowledge_segments WHERE meta->>'document_id'=%s AND meta->>'batch_id'=%s",
            (str(input.documentId), str(input.processingId)),
        )
        records = []
        for segment in segments:
            metadata = {key: value for key, value in segment.items() if key != "content"}
            metadata.update({
                "organization_id": str(input.organizationId), "knowledge_base_id": str(input.knowledgeBaseId),
                "document_id": str(input.documentId), "batch_id": str(input.processingId),
            })
            records.append((str(uuid5(input.processingId, str(segment["position"]))), segment["content"], Jsonb(metadata)))
        with connection.cursor() as cursor:
            cursor.executemany("INSERT INTO public.knowledge_segments (id,content,meta) VALUES (%s,%s,%s)", records)
    return {"segmentCount": len(segments), "stale": False}


@router.post("/process")
# 接收流式上传原件，处理临时文件在所有退出路径中清理。
def process_file(metadata: str = Form(), file: UploadFile = File()) -> dict:
    try:
        input = ProcessInput.model_validate_json(metadata)
    except ValueError:
        raise HTTPException(422, detail={"code": "invalid_request", "stage": "none"}) from None
    stage = "extracting"
    try:
        with TemporaryDirectory(prefix="cervi-knowledge-") as directory:
            path = Path(directory) / ("source" + Path(file.filename or "").suffix.lower())
            with path.open("wb") as target:
                while data := file.file.read(1024 * 1024):
                    target.write(data)
            set_stage(input, stage)
            documents = convert_file(path)
            stage = "splitting"
            set_stage(input, stage)
            segments = split_documents(documents, input.chunkLength, input.chunkOverlap)
            stage = "publishing"
            set_stage(input, stage)
            return save_segments(input, segments)
    except HTTPException:
        raise
    except psycopg.Error:
        logger.warning("知识分段存储失败 document_id=%s stage=%s", input.documentId, stage, exc_info=True)
        raise HTTPException(503, detail={"code": "service_failed", "stage": stage}) from None
    except Exception:
        logger.warning("知识原件处理失败 document_id=%s stage=%s", input.documentId, stage, exc_info=True)
        raise HTTPException(422, detail={"code": "parse_failed", "stage": stage}) from None


@router.post("/segments")
# 以同一数据库快照核验来源、定位锚点并返回一页正文。
def list_segments(input: SegmentInput) -> dict:
    if input.anchorSegmentId and input.page:
        raise HTTPException(422, detail={"code": "invalid_request"})
    scope = (str(input.organizationId), str(input.knowledgeBaseId), str(input.documentId), str(input.segmentBatchId))
    condition = "meta->>'organization_id'=%s AND meta->>'knowledge_base_id'=%s AND meta->>'document_id'=%s AND meta->>'batch_id'=%s"
    with psycopg.connect(os.environ["PG_CONN_STR"], row_factory=dict_row) as connection:
        connection.execute("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY")
        current = connection.execute(
            "SELECT kd.segment_count FROM knowledge_documents kd JOIN knowledge_bases kb ON kb.id=kd.knowledge_base_id "
            "WHERE kb.organization_id=%s AND kb.id=%s AND kd.id=%s AND kd.segment_batch_id=%s",
            scope,
        ).fetchone()
        if not current:
            raise HTTPException(409, detail={"code": "segment_stale"})
        page, position = input.page or 1, 0
        if input.anchorSegmentId:
            anchor = connection.execute(
                f"SELECT (meta->>'position')::int AS position FROM public.knowledge_segments WHERE {condition} AND id=%s",
                (*scope, str(input.anchorSegmentId)),
            ).fetchone()
            if not anchor:
                raise HTTPException(409, detail={"code": "segment_stale"})
            position = anchor["position"]
            # 按实际记录数定位页码，不依赖分段序号永久连续。
            preceding = connection.execute(
                f"SELECT count(*) AS count FROM public.knowledge_segments WHERE {condition} AND (meta->>'position')::int < %s",
                (*scope, position),
            ).fetchone()["count"]
            page = preceding // input.pageSize + 1
        rows = connection.execute(
            f"SELECT id,content,meta FROM public.knowledge_segments WHERE {condition} "
            "ORDER BY (meta->>'position')::int,id LIMIT %s OFFSET %s",
            (*scope, input.pageSize, (page - 1) * input.pageSize),
        ).fetchall()
    return {
        "segmentBatchId": str(input.segmentBatchId), "page": page, "pageSize": input.pageSize,
        "total": current["segment_count"], "anchorSegmentId": str(input.anchorSegmentId or ""), "anchorPosition": position,
        "segments": [{
            "id": row["id"], "position": row["meta"]["position"], "content": row["content"],
            "characterCount": row["meta"]["character_count"], "pageNumber": row["meta"].get("page_number"),
            "sourceLabel": row["meta"].get("source_label", ""),
        } for row in rows],
    }
