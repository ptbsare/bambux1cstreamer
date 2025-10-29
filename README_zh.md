# Bambu Lab X1C RTSP 视频流代理

[English README](README.md)

一个轻量级的、自托管的代理服务，用于转播拓竹 X1C 3D打印机的RSTPS视频流。本项目旨在解决打印机固件存在的RTSP并发连接数限制的Bug。它通过维持一个与打印机的持久连接，并将视频流通过WebRTC技术转播给多个客户端。

本仓库现在包含两个版本的视频流服务：
- **Go (默认, 推荐)**: 一个高性能版本，直接将 H.264 码流“直通”传输，不涉及转码。它能提供最佳的画质、最低的延迟和最少的CPU占用。
- **Python**: 最初的实现版本，它会解码视频流再重新编码。保留它是为了兼容性，但它的CPU占用更高，并可能导致画质损失。

## 功能特性

- **解决连接数限制**: 维持与打印机RSTPS流的单一稳定连接，允许无限数量的客户端通过本代理观看视频。
- **低延迟网页流**: 使用WebRTC技术将视频直接流式传输到浏览器，提供低延迟和高性能的观看体验。
- **高质量码流直通 (Go 版本)**: Go 版本避免了转码，将打印机原始的、无损的视频画质直接传输到您的浏览器。
- **自动重连**: 如果与打印机的连接意外断开，程序会自动尝试重新连接。
- **部署简单**: 为Docker进行了优化，支持 Go 和 Python 双后端，可以快速轻松地完成设置。
- **可嵌入的Web界面**: 提供一个干净、简洁的网页用于观看实时视频流，可以方便地通过iframe嵌入到Home Assistant等其他应用中。
- **集成CI/CD**: 包含一个GitHub Actions工作流，每次推送到`main`分支时，都会自动构建并发布一个`:beta`镜像到GitHub容器仓库(GHCR)。

## 如何使用

### 使用预构建的Docker镜像

我们在 GitHub 容器仓库 (GHCR) 上提供了两个主要的镜像标签：
- `latest`: 最新的稳定发行版，推荐大多数用户使用。
- `beta`: 基于`main`分支的最新代码自动构建，用于测试新功能。

1.  **运行 Docker 容器 (推荐):**
    请将 `PRINTER_IP` 和 `LAN_ACCESS_CODE` 替换为你的拓竹打印机的实际IP地址和局域网访问码。默认情况下，这将运行高性能的 Go 版本。
    ```bash
    docker run -d \
      --name bambu-streamer \
      -p 33002:33002 \
      -p 33005-33099:33005-33099/udp \
      -e RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1" \
      --restart unless-stopped \
      ghcr.io/ptbsare/bambux1cstreamer:latest
    ```

2.  **运行旧版 Python 版本:**
    将 `STREAMER_VERSION` 环境变量设置为 `python`。
    ```bash
    docker run -d \
      --name bambu-streamer-python \
      -p 33002:33002 \
      -p 33005-33099:33005-33099/udp \
      -e STREAMER_VERSION="python" \
      -e RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1" \
      --restart unless-stopped \
      ghcr.io/ptbsare/bambux1cstreamer:latest
    ```

3.  **观看视频流:**
    打开你的浏览器，访问 `http://<你的Docker主机IP>:33002` 即可。

### 从源码构建

1.  **构建 Docker 镜像:**
    这将构建一个包含 Go 和 Python 两个应用的多平台镜像。
    ```bash
    docker build -t bambux1cstreamer .
    ```

2.  **如上所示运行容器**, 但请将 `ghcr.io/ptbsare/bambux1cstreamer:latest` 替换为 `bambux1cstreamer`。

## 配置 (环境变量)

| 变量名                    | 描述                                                                                       | 默认值                                                                     |
|---------------------------|--------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| `RTSP_URL`                | **必需**. 你的拓竹打印机视频流的完整RSTPS地址。                                            | `rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1` (占位符)      |
| `WEB_PORT`                | Web服务在容器内部监听的端口。                                                              | `33002`                                                                    |
| `STREAMER_VERSION`        | 要运行的服务版本，可选值为 `go` 或 `python`。                                              | `go`                                                                       |
| `WEBRTC_UDP_PORT_MIN`     | WebRTC 连接使用的最小 UDP 端口。                                                           | `33005`                                                                    |
| `WEBRTC_UDP_PORT_MAX`     | WebRTC 连接使用的最大 UDP 端口。                                                           | `33099`                                                                    |
| `WEBRTC_LISTEN_ADDRESS`   | WebRTC 监听的 IP 地址。留空表示监听所有网络接口 (IPv4/IPv6)。                                | `""` (空字符串)                                                            |

## 防火墙与网络配置

为了让 WebRTC 正常工作，你必须暴露其用于对等连接 (peer-to-peer) 的 UDP 端口范围。默认情况下，端口范围是 `33005-33099`。

-   **Docker**: 运行容器时，你必须使用 `-p 33005-33099:33005-33099/udp` 参数来映射此 UDP 端口范围。
-   **防火墙**: 你还必须确保你的主机防火墙允许此端口范围的入站 UDP 流量。

以下是在一台使用 `ufw` 防火墙的 Linux 服务器上允许此端口范围的示例：
```bash
sudo ufw allow 33005:33099/udp
sudo ufw reload
```

## 反向代理配置 (公网访问)

如果你希望通过域名从公网访问视频流，你需要使用像 Nginx 这样的反向代理。**关键点：仅仅代理 Web 端口 (例如 33002) 是不够的。** 这样做只能让你看到网页界面，但视频流本身将无法连接。

这是因为 WebRTC 需要直接的 UDP 连接来传输媒体流。当服务位于 NAT 或防火墙后面时，就需要同时代理 UDP 端口范围。

### Nginx 配置 (包含 UDP Stream 代理)

你需要配置 Nginx 同时处理用于网页的 HTTP 流量和用于 WebRTC 媒体的 UDP 流量。这需要使用 Nginx 的 `stream` 模块，该模块可能需要在编译时手动启用 (`--with-stream`)。

这是一个完整的 `nginx.conf` 配置示例：

```nginx
# /etc/nginx/nginx.conf
load_module /usr/lib/nginx/modules/ngx_stream_module.so;

# 在 http 块的同级添加 stream 块
stream {
    # 代理 WebRTC UDP 端口范围
    server {
        listen 33005-33099 udp;
        proxy_pass 127.0.0.1:$server_port; # 转发到本机的相同端口
        proxy_responses 0;
    }
}

http {
    # ... 其他 http 设置 ...

    server {
        listen 80;
        server_name your_domain.com;

        location / {
            proxy_pass http://127.0.0.1:33002; # 代理到 Web 界面
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "Upgrade";
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }
    }
}
```

### 流量示意图

此图展示了流量如何从公网客户端通过 Nginx 流向 streamer 服务：

```
   公网客户端
      |
      |-- 1. HTTPS/WSS (TCP 443) --> Nginx (your_domain.com)
      |                                |
      |                                +-- proxy_pass --> bambux1cstreamer (TCP 33002)
      |                                                     (网页与信令)
      |
      |-- 2. WebRTC 媒体流 (UDP) ---> Nginx (公网 IP, UDP 33005-33099)
                                       |
                                       +-- stream proxy_pass --> bambux1cstreamer (UDP 33005-33099)
                                                                 (视频/音频流)
```

**重要提示:**
- **防火墙**: 确保你的 Nginx 服务器上的防火墙允许 HTTP/HTTPS 端口 (例如 80/443) 和 UDP 端口范围 (`33005-33099`) 的入站流量。
- **Docker 网络**: 如果 Nginx 运行在另一个 Docker 容器中，请确保它可以访问到 `bambux1cstreamer` 容器。推荐使用共享的 Docker 网络。
- **`WEBRTC_LISTEN_ADDRESS`**: 使用反向代理时，你可能需要将 `WEBRTC_LISTEN_ADDRESS` 设置为你的服务器的公网 IP 地址，以便生成正确的 ICE 候选地址。
