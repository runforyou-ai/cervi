"""验证真实 PostgreSQL 分段存储、分页定位与失效批次。"""

import os
import unittest
from uuid import uuid4, uuid5

import psycopg
from fastapi import FastAPI, HTTPException
from fastapi.testclient import TestClient
from psycopg.conninfo import make_conninfo

from knowledge import ProcessInput, SegmentInput, list_segments, router, save_segments


class StorageTests(unittest.TestCase):
    """使用工作区测试数据库验证持久化契约。"""

    def setUp(self):
        """为每个测试建立独立的业务来源。"""
        self.connection_string = make_conninfo(
            host=os.environ["TEST_POSTGRES_HOST"], port=os.environ["TEST_POSTGRES_PORT"],
            user=os.environ["TEST_POSTGRES_USER"], password=os.environ["TEST_POSTGRES_PASSWORD"],
            dbname=os.environ["TEST_POSTGRES_DB"], sslmode=os.environ["TEST_POSTGRES_SSLMODE"],
        )
        os.environ["PG_CONN_STR"] = self.connection_string
        self.input = ProcessInput(organizationId=uuid4(), knowledgeBaseId=uuid4(), documentId=uuid4(), processingId=uuid4(), chunkLength=512, chunkOverlap=50)
        self.segments = [{"position": i, "content": f"分段 {i}", "character_count": len(f"分段 {i}"), "page_number": None, "source_label": ""} for i in range(1, 106)]
        with psycopg.connect(self.connection_string) as connection:
            connection.execute("INSERT INTO knowledge_bases(id,organization_id,created_by_user_id,name,category) VALUES (%s,%s,%s,%s,'standard')", (self.input.knowledgeBaseId,self.input.organizationId,uuid4(),str(uuid4())))
            connection.execute("INSERT INTO knowledge_documents(id,knowledge_base_id,group_id,file_id,created_by_user_id,status,processing_id) VALUES (%s,%s,%s,%s,%s,'fetching',%s)", (self.input.documentId,self.input.knowledgeBaseId,uuid4(),uuid4(),uuid4(),self.input.processingId))

    def tearDown(self):
        """清除本例来源和分段。"""
        with psycopg.connect(self.connection_string) as connection:
            connection.execute("DELETE FROM public.knowledge_segments WHERE meta->>'document_id'=%s", (str(self.input.documentId),))
            connection.execute("DELETE FROM knowledge_documents WHERE id=%s", (self.input.documentId,))
            connection.execute("DELETE FROM knowledge_bases WHERE id=%s", (self.input.knowledgeBaseId,))

    def publish(self):
        """模拟 Go 在远端完整落库后的批次发布。"""
        with psycopg.connect(self.connection_string) as connection:
            connection.execute("UPDATE knowledge_documents SET status='succeeded',segment_batch_id=%s,segment_count=105 WHERE id=%s", (self.input.processingId,self.input.documentId))

    def query(self, **kwargs):
        """构造当前来源的分页输入。"""
        return SegmentInput(organizationId=self.input.organizationId,knowledgeBaseId=self.input.knowledgeBaseId,documentId=self.input.documentId,segmentBatchId=self.input.processingId,**kwargs)

    def test_pagination_anchor_and_replay(self):
        """重复写入不增加记录，任意锚点直接定位且相邻页无遗漏。"""
        for _ in range(2):
            self.assertEqual(save_segments(self.input,self.segments)["segmentCount"],105)
        with self.assertRaises(HTTPException):
            list_segments(self.query(page=1))
        self.publish()
        for position, page_number in [(1,1),(20,1),(21,2),(87,5),(105,6)]:
            anchor = uuid5(self.input.processingId,str(position))
            page=list_segments(self.query(anchorSegmentId=anchor))
            self.assertEqual(page["page"],page_number)
            self.assertEqual(page["anchorPosition"],position)
            self.assertIn(str(anchor),[segment["id"] for segment in page["segments"]])
        pages=[list_segments(self.query(page=n))["segments"] for n in range(1,7)]
        self.assertEqual([segment["position"] for page in pages for segment in page],list(range(1,106)))
        self.assertEqual(list_segments(self.query(page=7))["segments"],[])
        self.assertTrue(save_segments(self.input,self.segments)["stale"])

    def test_foreign_and_stale_queries(self):
        """错误企业、错误文档和旧批次均不能读取正文。"""
        save_segments(self.input,self.segments)
        self.publish()
        for field in ["organizationId","knowledgeBaseId","documentId","segmentBatchId"]:
            query=self.query(page=1).model_copy(update={field:uuid4()})
            with self.assertRaises(HTTPException) as stale:
                list_segments(query)
            self.assertEqual(stale.exception.detail["code"],"segment_stale")
        with self.assertRaises(HTTPException):
            list_segments(self.query(anchorSegmentId=uuid4()))
        with psycopg.connect(self.connection_string) as connection:
            connection.execute("DELETE FROM knowledge_documents WHERE id=%s",(self.input.documentId,))
        self.assertTrue(save_segments(self.input,self.segments)["stale"])
        with self.assertRaises(HTTPException):
            list_segments(self.query(page=1))

    def test_http_upload(self):
        """通过真正的 multipart 接口完成解析、保存及分页读取。"""
        app=FastAPI()
        app.include_router(router)
        with TestClient(app) as client:
            text="合同金额 1,234.50 元，不得退款。\n"*1500
            response=client.post("/knowledge/process",data={"metadata":self.input.model_dump_json()},files={"file":("合同.txt",text.encode(),"text/plain")})
            self.assertEqual(response.status_code,200,response.text)
            count=response.json()["segmentCount"]
            self.assertGreater(count,20)
            with psycopg.connect(self.connection_string) as connection:
                connection.execute("UPDATE knowledge_documents SET status='succeeded',segment_batch_id=%s,segment_count=%s WHERE id=%s",(self.input.processingId,count,self.input.documentId))
            page=client.post("/knowledge/segments",json=self.query(page=1).model_dump(mode="json"))
            self.assertEqual(page.status_code,200,page.text)
            self.assertEqual(len(page.json()["segments"]),20)

    def test_reprocessing_preserves_published_batch(self):
        """重试写入新批次时旧分段仍可读，旧任务不能修改新任务状态或正文。"""
        from knowledge import set_stage
        save_segments(self.input, self.segments)
        self.publish()
        replacement = self.input.model_copy(update={"processingId":uuid4(), "chunkLength":256})
        with psycopg.connect(self.connection_string) as connection:
            connection.execute("UPDATE knowledge_documents SET processing_id=%s,status='queued' WHERE id=%s",(replacement.processingId,self.input.documentId))
        set_stage(self.input, "extracting")
        self.assertTrue(save_segments(self.input, self.segments)["stale"])
        with psycopg.connect(self.connection_string) as connection:
            self.assertEqual(connection.execute("SELECT status FROM knowledge_documents WHERE id=%s",(self.input.documentId,)).fetchone()[0], "queued")
        set_stage(replacement, "publishing")
        save_segments(replacement, self.segments[:3])
        self.assertEqual(list_segments(self.query(page=1))["total"],105)
        with psycopg.connect(self.connection_string) as connection:
            count = connection.execute("SELECT count(*) FROM public.knowledge_segments WHERE meta->>'document_id'=%s",(str(self.input.documentId),)).fetchone()[0]
        self.assertEqual(count,108)
