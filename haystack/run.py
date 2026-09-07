# /// script
# requires-python = ">=3.10"
# dependencies = ["hayhooks==1.24.0", "haystack-ai==3.1.1", "pgvector-haystack==6.6.0"]
# ///
"""读取当前工作区环境，启动 Haystack 向量验证 HTTP 服务。"""

import os
from pathlib import Path

from psycopg.conninfo import make_conninfo


if __name__ == "__main__":
    os.environ["PG_CONN_STR"] = make_conninfo(
        host=os.environ["POSTGRES_HOST"],
        port=os.environ["POSTGRES_PORT"],
        user=os.environ["POSTGRES_USER"],
        password=os.environ["POSTGRES_PASSWORD"],
        dbname=os.environ["POSTGRES_DB"],
        sslmode=os.environ["POSTGRES_SSLMODE"],
    )
    os.environ["HAYHOOKS_PIPELINES_DIR"] = str(Path(__file__).parent / "pipelines")

    # Hayhooks 导入时读取环境配置。
    from hayhooks import create_app, run_app

    run_app(create_app(), host="127.0.0.1", port=int(os.environ["HAYSTACK_PORT"]))
