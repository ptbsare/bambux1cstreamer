# Bambu Lab X1C RTSP Stream Proxy

[中文文档](README_zh.md)

A lightweight, self-hosted proxy to re-stream the RTSPS video from a Bambu Lab X1C 3D printer. This application addresses the printer's firmware bug that limits the number of concurrent RTSP connections by maintaining a single, persistent connection and re-streaming it to multiple clients via WebRTC.

## Features

- **Solves Connection Limit**: Maintains a single, stable connection to the printer's RTSPS stream, allowing unlimited clients to view the feed through the proxy.
- **Low Latency Web Stream**: Uses WebRTC to stream video directly to browsers, offering low latency and high performance.
- **Automatic Reconnection**: If the connection to the printer is lost, the application will automatically try to reconnect.
- **Easy Deployment**: Optimized for Docker, allowing for quick and easy setup.
- **Embeddable Web Interface**: Provides a clean, simple web page to view the live stream, which can be easily embedded in other applications like Home Assistant using an iframe.
- **CI/CD Ready**: Includes a GitHub Actions workflow to automatically build and publish a `beta` image to Docker Hub on every push to the `main` branch.

## How to Use

### Using the Pre-built Docker Image

The latest beta image is automatically built and pushed to Docker Hub.

1.  **Run the Docker container:**
    Replace `PRINTER_IP` and `LAN_ACCESS_CODE` with your Bambu Lab printer's IP address and access code.
    ```bash
    docker run -d \
      --name bambu-streamer \
      -p 33002:33002 \
      -e RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1" \
      --restart unless-stopped \
      ghcr.io/ptbsare/bambux1cstreamer:beta
    ```

2.  **View the stream:**
    Open your web browser and navigate to `http://<your_docker_host_ip>:33002`.

### Building from Source

1.  **Build the Docker image:**
    ```bash
    docker build -t bambu-streamer .
    ```

2.  **Run the container as shown above**, but replace `ghcr.io/ptbsare/bambux1cstreamer:beta` with `bambu-streamer`.

## Configuration (Environment Variables)

| Variable    | Description                                                                 | Default                               |
|-------------|-----------------------------------------------------------------------------|---------------------------------------|
| `RTSP_URL`  | **Required**. The full RTSPS URL of your Bambu Lab printer's live stream.     | `rstsps://your_printer_ip/bbl_liveview/stream` (placeholder) |
| `WEB_PORT`  | The port on which the web server will listen inside the container.          | `33002`                               |
