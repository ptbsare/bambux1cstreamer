# --- Build Stage ---
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the Go source code
COPY main.go .
COPY static ./static

# Build the Go application for a minimal final image
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /bambux1cstreamer-go main.go


# --- Final Stage ---
FROM alpine:latest

WORKDIR /app

# Copy the compiled Go application from the builder stage
COPY --from=builder /bambux1cstreamer-go /usr/local/bin/bambux1cstreamer-go

# Copy the static files for the web interface
COPY --from=builder /app/static ./static

# Set default environment variables
ENV WEB_PORT=33003
ENV RTSP_PROXY_PORT=8554
ENV RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1"
ENV WEBRTC_UDP_PORT_MIN=33005
ENV WEBRTC_UDP_PORT_MAX=33099

# Expose the default ports
EXPOSE ${WEB_PORT}
EXPOSE ${RTSP_PROXY_PORT}
EXPOSE ${WEBRTC_UDP_PORT_MIN}-${WEBRTC_UDP_PORT_MAX}/udp

# Run the application
ENTRYPOINT ["/usr/local/bin/bambux1cstreamer-go"]