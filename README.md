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
      -p 33005-33099:33005-33099/udp \
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
      -p 33005-33099:33005-33099/udp \
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

| Variable                  | Description                                                                                                 | Default                                                                    |
|---------------------------|-------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| `RTSP_URL`                | **Required**. The full RTSPS URL of your Bambu Lab printer's live stream.                                   | `rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1` (placeholder) |
| `WEB_PORT`                | The port on which the web server will listen inside the container.                                          | `33002`                                                                    |
| `STREAMER_VERSION`        | The version of the streamer to run. Can be `go` or `python`.                                                | `go`                                                                       |
| `WEBRTC_UDP_PORT_MIN`     | The minimum UDP port for WebRTC connections.                                                                | `33005`                                                                    |
| `WEBRTC_UDP_PORT_MAX`     | The maximum UDP port for WebRTC connections.                                                                | `33099`                                                                    |

## Firewall and Network Configuration

For WebRTC to work correctly, you must expose the UDP port range used for peer-to-peer connections. By default, this is `33005-33099`.

-   **Docker**: When running the container, you must map this UDP port range using the `-p 33005-33099:33005-33099/udp` flag.
-   **Firewall**: You must also ensure that your host's firewall allows incoming UDP traffic on this port range.

Here is an example of how to allow this range on a Linux server using `ufw`:
```bash
sudo ufw allow 33005:33099/udp
sudo ufw reload
```

## Reverse Proxy Configuration (for Public Internet Access)

If you want to access the video stream from the public internet through a reverse proxy like Nginx, please note that **simply proxying the web port (e.g., 33002) is not enough.**

This is because the WebRTC protocol establishes a direct media connection. While the web page and signaling will load through your proxy, the video stream itself will attempt to connect directly to the IP address of the host running `bambux1cstreamer`.

Therefore, in addition to setting up a reverse proxy for the web port, you must also **open the WebRTC UDP port range (e.g., 33005-33099) on the firewall of the host machine where the `bambux1cstreamer` container is running.** This allows clients to establish a direct connection for the video feed. 

