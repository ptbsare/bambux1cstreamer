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

如果你希望通过 Nginx 等反向代理从公网访问视频流，请注意：**仅仅代理 Web 端口 (例如 33002) 是不够的。**

这是因为 WebRTC 协议会建立一个直接的媒体连接。虽然网页和信令数据可以通过你的代理加载，但视频流本身会尝试直接连接到运行 `bambux1cstreamer` 的主机的 IP 地址。

因此，除了为 Web 端口设置反向代理之外，你还必须**在运行 `bambux1cstreamer` 容器的主机防火墙上，放行 WebRTC 的 UDP 端口范围 (例如 33005-33099)。** 这使得客户端可以为视频流建立直接连接。
