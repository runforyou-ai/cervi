"""在 haystack schema 中写入示例向量并验证 pgvector 检索。"""

from hayhooks import BasePipelineWrapper
from haystack import Document, Pipeline
from haystack.document_stores.types import DuplicatePolicy
from haystack_integrations.components.retrievers.pgvector import PgvectorEmbeddingRetriever
from haystack_integrations.document_stores.pgvector import PgvectorDocumentStore


class PipelineWrapper(BasePipelineWrapper):
    def setup(self) -> None:
        """连接 PostgreSQL，写入两条固定的三维示例向量。"""
        document_store = PgvectorDocumentStore(
            schema_name="haystack",
            table_name="pgvector_check",
            embedding_dimension=3,
            create_extension=False,
        )
        document_store.write_documents(
            [
                Document(id="vector-x", content="向量 X", embedding=[1.0, 0.0, 0.0]),
                Document(id="vector-y", content="向量 Y", embedding=[0.0, 1.0, 0.0]),
            ],
            policy=DuplicatePolicy.OVERWRITE,
        )
        self.pipeline = Pipeline()
        self.pipeline.add_component(
            "retriever", PgvectorEmbeddingRetriever(document_store=document_store, top_k=1)
        )

    def run_api(self, query_embedding: list[float]) -> list[dict]:
        """返回与查询向量最接近的示例文档及相似度。"""
        result = self.pipeline.run({"retriever": {"query_embedding": query_embedding}})
        return [
            {"id": document.id, "content": document.content, "score": document.score}
            for document in result["retriever"]["documents"]
        ]
