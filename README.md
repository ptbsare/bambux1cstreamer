# Bambu Lab X1C RTSP Stream Proxy

[中文文档](README_zh.md)

A lightweight, self-hosted proxy to re-stream the RTSPS video from a Bambu Lab X1C 3D printer. This application addresses the printer's firmware bug that limits the number of concurrent RTSP connections by maintaining a single, persistent connection and re-streaming it to multiple clients via WebRTC.

This repository now includes two versions of the streamer:
- **Go (Default, Recommended)**: A high-performance version that directly passes through the H.264 stream without transcoding. It offers the best quality, lowest latency, and minimal CPU usage.
- **Python**: The original implementation that decodes and re-encodes the stream. It is kept for compatibility but has higher CPU usage and potential for quality loss.

## Features

- **Solves Connection Limit**: Maintains a single, stable connection to the printer's RTSPS stream, allowing unlimited clients to view the feed through the proxy.
- **Low Latency Web Stream**: Uses WebRTC to stream video directly to browsers, offering low latency and high performance.
- **High-Quality Passthrough (Go version)**: The Go version avoids transcoding, delivering the original, lossless video quality from the printer to your browser.
- **Automatic Reconnection**: If the connection to the printer is lost, the application will automatically try to reconnect.
- **Easy Deployment**: Optimized for Docker, allowing for quick and easy setup with support for both Go and Python backends.
- **Embeddable Web Interface**: Provides a clean, simple web page to view the live stream, which can be easily embedded in other applications like Home Assistant using an iframe.
- **CI/CD Ready**: Includes a GitHub Actions workflow to automatically build and publish a `beta` image to GitHub Container Registry (GHCR) on every push to the `main` branch.

## How to Use

### Using Pre-built Docker Images

We provide two main image tags on GitHub Container Registry:
- `latest`: The latest stable release. Recommended for most users.
- `beta`: Built automatically from the latest commit on the `main` branch. Use this for testing new features.

1.  **Run the Docker container (Recommended):**
    Replace `PRINTER_IP` and `LAN_ACCESS_CODE` with your Bambu Lab printer's IP address and access code. By default, this will run the high-performance Go version.
    ```bash
    docker run -d \
      --name bambu-streamer \
      -p 33002:33002 \
      -e RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1" \
      --restart unless-stopped \
      ghcr.io/ptbsare/bambux1cstreamer:latest
    ```

2.  **To run the legacy Python version:**
    Set the `STREAMER_VERSION` environment variable to `python`.
    ```bash
    docker run -d \
      --name bambu-streamer-python \
      -p 33002:33002 \
      -e STREAMER_VERSION="python" \
      -e RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1" \
      --restart unless-stopped \
      ghcr.io/ptbsare/bambux1cstreamer:latest
    ```

3.  **View the stream:**
    Open your web browser and navigate to `http://<your_docker_host_ip>:33002`.

### Building from Source

1.  **Build the Docker image:**
    This will build a multi-platform image containing both the Go and Python applications.
    ```bash
    docker build -t bambux1cstreamer .
    ```

2.  **Run the container as shown above**, but replace `ghcr.io/ptbsare/bambux1cstreamer:latest` with `bambu-streamer`.

## Configuration (Environment Variables)

| Variable           | Description                                                                 | Default                               |
|--------------------|-----------------------------------------------------------------------------|---------------------------------------|
| `RTSP_URL`         | **Required**. The full RTSPS URL of your Bambu Lab printer's live stream.     | `rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1` (placeholder) |
| `WEB_PORT`         | The port on which the web server will listen inside the container.          | `33002`                               |
| `STREAMER_VERSION` | The version of the streamer to run. Can be `go` or `python`.                | `go`                                  |
