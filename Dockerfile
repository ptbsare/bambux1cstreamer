# --- Build Stage for Go ---
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the Go source code
COPY main.go .
COPY static ./static

# Build the Go application
# CGO_ENABLED=0 is important for creating a static binary
# -ldflags "-s -w" strips debug information to reduce binary size
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /bambux1cstreamer-go main.go


# --- Final Stage for Python and Go ---
FROM python:3.13-slim

WORKDIR /app

# Set environment variables for Python
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# Install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the Python application
COPY app.py .

# Copy the compiled Go application from the builder stage
COPY --from=builder /bambux1cstreamer-go /usr/local/bin/bambux1cstreamer-go

# Copy the static files for the Go application
COPY --from=builder /app/static ./static

# Copy the entrypoint script and make it executable
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Set default streamer version to "go"
ENV STREAMER_VERSION=go
# Set default web port
ENV WEB_PORT=33002
# Set a placeholder RTSP URL. Users MUST override this.
ENV RTSP_URL="rtsps://bblp:LAN_ACCESS_CODE@PRINTER_IP:322/streaming/live/1"

# Expose the default port
EXPOSE ${WEB_PORT}

# The entrypoint script will decide which application to run
ENTRYPOINT ["./entrypoint.sh"]