FROM python:3.13-slim-trixie

# exiftool 提供图像元数据读取，ffmpeg 供 markitdown[all] 引入的 pydub 使用。
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg libimage-exiftool-perl \
    && rm -rf /var/lib/apt/lists/*

# markitdown[all] 安装 pptx、docx、xlsx、xls、pdf、outlook、音频、YouTube 与 Azure 全部 extra。
RUN pip install --no-cache-dir \
    "markitdown[all]==0.1.7" \
    "fastapi==0.141.1" \
    "uvicorn==0.52.4" \
    "python-multipart==0.0.32"

ENV PYTHONDONTWRITEBYTECODE=1
WORKDIR /app
COPY server.py /app/server.py
USER nobody
EXPOSE 8000
CMD ["uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8000"]
