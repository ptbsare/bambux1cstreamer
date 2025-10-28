# Bambu Lab X1C RTSP 视频流代理

[English README](README.md)

一个轻量级的、自托管的代理服务，用于转播拓竹 X1C 3D打印机的RSTPS视频流。本项目旨在解决打印机固件存在的RTSP并发连接数限制的Bug。它通过维持一个与打印机的持久连接，并将视频流通过WebRTC技术转播给多个客户端。

## 功能特性

- **解决连接数限制**: 维持与打印机RSTPS流的单一稳定连接，允许无限数量的客户端通过本代理观看视频。
- **低延迟网页流**: 使用WebRTC技术将视频直接流式传输到浏览器，提供低延迟和高性能的观看体验。
- **自动重连**: 如果与打印机的连接意外断开，程序会自动尝试重新连接。
- **部署简单**: 为Docker进行了优化，可以快速轻松地完成设置。
- **可嵌入的Web界面**: 提供一个干净、简洁的网页用于观看实时视频流，可以方便地通过iframe嵌入到Home Assistant等其他应用中。
- **集成CI/CD**: 包含一个GitHub Actions工作流，每次推送到`main`分支时，都会自动构建并发布一个`:beta`镜像到GitHub容器仓库(GHCR)。

## 如何使用

### 使用预构建的Docker镜像

我们在 GitHub 容器仓库 (GHCR) 上提供了两个主要的镜像标签：
- `latest`: 最新的稳定发行版，推荐大多数用户使用。
- `beta`: 基于`main`分支的最新代码自动构建，用于测试新功能。

1.  **运行 Docker 容器 (推荐):**
    请将 `PRINTER_IP` 和 `LAN_ACCESS_CODE` 替换为你的拓竹打印机的实际IP地址和局域网访问码。
    ```bash
    docker run -d \
      --name bambu-streamer \
      -p 33002:33002 \
      -e RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1" \
      --restart unless-stopped \
      ghcr.io/ptbsare/bambux1cstreamer:latest
    ```

2.  **观看视频流:**
    打开你的浏览器，访问 `http://<你的Docker主机IP>:33002` 即可。

### 从源码构建

1.  **构建 Docker 镜像:**
    ```bash
    docker build -t bambu-streamer .
    ```

2.  **如上所示运行容器**, 但请将 `ghcr.io/ptbsare/bambux1cstreamer:beta` 替换为 `bambu-streamer`。

## 配置 (环境变量)

| 变量名      | 描述                                                              | 默认值                                               |
|-------------|-------------------------------------------------------------------|------------------------------------------------------|
| `RTSP_URL`  | **必需**. 你的拓竹打印机视频流的完整RSTPS地址。                     | `rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1` (占位符) |
| `WEB_PORT`  | Web服务在容器内部监听的端口。                                     | `33002`                                              |
