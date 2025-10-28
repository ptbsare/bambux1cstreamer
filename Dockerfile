# --- Build Stage ---
FROM python:3.13-slim as builder

# 安装 uv
RUN pip install uv

# 创建虚拟环境
WORKDIR /app
RUN uv venv

# 安装系统依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    libgl1 \
    && rm -rf /var/lib/apt/lists/*

# 安装 Python 依赖到虚拟环境中
COPY requirements.txt .
RUN . .venv/bin/activate && uv pip install --no-cache-dir -r requirements.txt

# --- Final Stage ---
FROM python:3.13-slim

WORKDIR /app

# 从构建阶段复制虚拟环境
COPY --from=builder /app/.venv .venv

# 复制应用程序代码
COPY app.py .

# 将虚拟环境的 bin 目录添加到 PATH
ENV PATH="/app/.venv/bin:$PATH"

# 暴露 Web 服务的端口
EXPOSE 33002

# 设置默认的环境变量
ENV RTSP_URL="rstsps://your_printer_ip/bbl_liveview/stream"
ENV WEB_PORT="33002"

# 使用虚拟环境中的 python 启动应用程序
CMD ["python", "app.py"]