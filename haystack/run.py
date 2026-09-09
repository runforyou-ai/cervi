# /// script
# requires-python = ">=3.10"
# dependencies = [
#   "hayhooks==1.24.0", "haystack-ai==3.1.1", "pgvector-haystack==6.6.0",
#   "pypdf==6.8.0", "python-docx==1.2.0", "python-pptx==1.0.2", "openpyxl==3.1.5",
#   "pandas==3.0.1", "trafilatura==2.0.0", "tabulate==0.9.0", "python-multipart==0.0.22",
# ]
# ///
"""读取当前工作区环境，启动 Haystack 知识处理 HTTP 服务。"""

import os
import sys
import unittest
from pathlib import Path

from psycopg.conninfo import make_conninfo


if __name__ == "__main__":
    if "--test" in sys.argv or "--test-parsing" in sys.argv:
        suite = unittest.defaultTestLoader.discover(str(Path(__file__).parent / "tests"), pattern="test_processing.py" if "--test-parsing" in sys.argv else "test*.py")
        sys.exit(0 if unittest.TextTestRunner(verbosity=2).run(suite).wasSuccessful() else 1)

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

    from knowledge import router

    app = create_app()
    app.include_router(router)
    run_app(app, host="127.0.0.1", port=int(os.environ["HAYSTACK_PORT"]))
