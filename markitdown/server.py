"""把上传原件转换为 Markdown 正文的无状态 HTTP 服务。"""

import logging
from pathlib import Path
from tempfile import TemporaryDirectory

from fastapi import FastAPI, File, HTTPException, UploadFile
from markitdown import MarkItDown

# markitdown 的音频转换器把内容提交到外部语音识别服务，转换入口拒绝其接受的后缀。
AUDIO_SUFFIXES = frozenset({".wav", ".mp3", ".m4a", ".mp4"})

app = FastAPI()
logger = logging.getLogger("uvicorn.error")
converter = MarkItDown()


@app.get("/status")
def status() -> dict:
    """返回服务可用状态。"""
    return {"status": "ok"}


@app.post("/convert")
def convert(file: UploadFile = File()) -> dict:
    """在临时目录中把上传原件转换为 Markdown 正文，退出时清理临时副本。"""
    suffix = Path(file.filename or "").suffix.lower()
    if suffix in AUDIO_SUFFIXES:
        raise HTTPException(415, detail={"code": "unsupported_file"})
    with TemporaryDirectory(prefix="cervi-convert-") as directory:
        path = Path(directory) / ("source" + suffix)
        with path.open("wb") as target:
            while data := file.file.read(1024 * 1024):
                target.write(data)
        try:
            return {"markdown": converter.convert(path).markdown}
        except Exception:
            logger.warning("原件转换失败 name=%s", file.filename, exc_info=True)
            raise HTTPException(422, detail={"code": "parse_failed"}) from None
