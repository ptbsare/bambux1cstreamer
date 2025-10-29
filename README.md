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
| `WEBRTC_LISTEN_ADDRESS`   | The IP address for WebRTC to listen on. Leave empty to listen on all interfaces (IPv4/IPv6).                | `""` (empty)                                                               |

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

If you want to access the video stream from the public internet through a domain name, you will need a reverse proxy like Nginx. **Crucially, simply proxying the web port (e.g., 33002) is not enough.** This will only allow you to see the web interface, but the video stream itself will fail to connect.

This is because WebRTC requires a direct UDP connection for media streaming. When behind a NAT or firewall, this requires proxying the UDP port range as well.

### Nginx Configuration with UDP Stream Proxy

You need to configure Nginx to handle both the HTTP traffic for the web page and the UDP traffic for the WebRTC media. This requires using the `stream` module in Nginx, which may need to be enabled during compilation (`--with-stream`).

Here is a complete configuration example for `nginx.conf`:

```nginx
# /etc/nginx/nginx.conf
load_module /usr/lib/nginx/modules/ngx_stream_module.so;
# Add this stream block at the same level as the http block
stream {
    # Proxy for the WebRTC UDP port range
    server {
        listen 33005-33099 udp;
        proxy_pass 127.0.0.1:$server_port; # Forward to the same port on localhost
        proxy_responses 0;
    }
}

http {
    # ... your other http settings ...

    server {
        listen 80;
        server_name your_domain.com;

        location / {
            proxy_pass http://127.0.0.1:33002; # Proxy to the web interface
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

### Traffic Flow Diagram

This diagram illustrates how the traffic flows from a public client to the streamer through Nginx:

```
   Public Client
        |
        |-- 1. HTTPS/WSS (TCP 443) --> Nginx (your_domain.com)
        |                                |
        |                                +-- proxy_pass --> bambux1cstreamer (TCP 33002)
        |                                                     (Web page and Signaling)
        |
        |-- 2. WebRTC Media (UDP) ----> Nginx (Public IP, UDP 33005-33099)
                                         |
                                         +-- stream proxy_pass --> bambux1cstreamer (UDP 33005-33099)
                                                                   (Video/Audio Stream)
```

**Important Considerations:**
- **Firewall**: Ensure your firewall on the Nginx server allows incoming traffic on both the HTTP/HTTPS port (e.g., 80/443) and the UDP port range (`33005-33099`).
- **Docker Network**: If Nginx is running in a separate Docker container, ensure it can reach the `bambux1cstreamer` container. Using a shared Docker network is recommended.
- **`WEBRTC_LISTEN_ADDRESS`**: When using a reverse proxy, you might need to set the `WEBRTC_LISTEN_ADDRESS` to your server's public IP address so that the correct ICE candidates are generated.
