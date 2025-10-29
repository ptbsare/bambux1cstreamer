#!/bin/sh

# Check the STREAMER_VERSION environment variable
if [ "$STREAMER_VERSION" = "python" ]; then
    echo "Starting Python version..."
    exec python app.py
else
    echo "Starting Go version (default)..."
    # The Go app serves static files from the ./static directory relative to where it's run
    # The Dockerfile places these in /app/static, and the WORKDIR is /app
    exec /usr/local/bin/bambux1cstreamer-go
fi