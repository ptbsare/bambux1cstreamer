import os
import asyncio
import cv2
import logging
import json
import time
import collections
import numpy as np
from fractions import Fraction
from aiohttp import web
from aiortc import RTCPeerConnection, RTCSessionDescription, VideoStreamTrack
from av import VideoFrame

# --- Configuration ---
RTSP_URL = os.environ.get("RTSP_URL", "rstsps://your_printer_ip/bbl_liveview/stream")
WEB_PORT = int(os.environ.get("WEB_PORT", 33002))
os.environ["OPENCV_FFMPEG_CAPTURE_OPTIONS"] = "rtsp_transport;tcp|tls_verify;0"

# --- Logging Setup ---
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger("bambu_streamer")
# Use DEBUG for more verbose output from aiortc and others
# logging.getLogger("aiortc").setLevel(logging.DEBUG)
# logging.getLogger("aioice").setLevel(logging.DEBUG)


class StreamManager:
    """
    Manages a single, persistent RTSP connection and distributes frames to listeners.
    This acts as the "Producer" in a Producer-Consumer model.
    """
    _instance = None

    def __new__(cls, *args, **kwargs):
        if not cls._instance:
            cls._instance = super(StreamManager, cls).__new__(cls)
        return cls._instance

    def __init__(self, rtsp_url):
        if not hasattr(self, '_initialized'):  # Prevent re-initialization
            self.rtsp_url = rtsp_url
            self._listeners = set()
            self._capture_task = None
            self._frame_buffer = collections.deque(maxlen=60) # Keep a buffer for new clients
            self._lock = asyncio.Lock()
            self._initialized = True
            logger.info("StreamManager initialized.")

    async def start(self):
        async with self._lock:
            if self._capture_task is None:
                self._capture_task = asyncio.create_task(self._capture_loop())
                logger.info("RTSP capture task started.")

    async def stop(self):
        async with self._lock:
            if self._capture_task:
                self._capture_task.cancel()
                try:
                    await self._capture_task
                except asyncio.CancelledError:
                    pass
                self._capture_task = None
                logger.info("RTSP capture task stopped.")

    async def register(self, track):
        async with self._lock:
            logger.info(f"Registering new track... Current listeners: {len(self._listeners)}")
            # Pre-fill the new track's personal queue with the historical buffer
            for frame in list(self._frame_buffer):
                try:
                    track.queue.put_nowait(frame)
                except asyncio.QueueFull:
                    # The track's queue is full, which is expected and fine.
                    # It means the client has a sufficient buffer to start.
                    break
            self._listeners.add(track)
            logger.info(f"Track registered. Total listeners: {len(self._listeners)}")

    async def unregister(self, track):
        async with self._lock:
            self._listeners.discard(track)
            logger.info(f"Track unregistered. Total listeners: {len(self._listeners)}")

    async def _capture_loop(self):
        loop = asyncio.get_running_loop()
        while True:
            try:
                logger.info(f"Connecting to RTSP stream: {self.rtsp_url}")
                cap = await loop.run_in_executor(None, lambda: cv2.VideoCapture(self.rtsp_url, cv2.CAP_FFMPEG))
                
                if not cap.isOpened():
                    logger.error("Failed to open RTSP stream. Retrying in 10 seconds...")
                    await asyncio.sleep(10)
                    continue

                logger.info("RTSP stream connected successfully.")
                while True:
                    # Read frame first
                    # Read frame and its original timestamp from the stream
                    ret, frame = await loop.run_in_executor(None, cap.read)
                    if not ret:
                        logger.warning("Failed to read frame from stream. Reconnecting...")
                        break
                    
                    rtsp_timestamp_ms = cap.get(cv2.CAP_PROP_POS_MSEC)
                    
                    # Add the frame and its original timestamp to our historical cache
                    self._frame_buffer.append((frame, rtsp_timestamp_ms))

                    # Broadcast the new frame to all active listeners
                    for track in self._listeners:
                        try:
                            if track.queue.full():
                                track.queue.get_nowait()
                            track.queue.put_nowait((frame, rtsp_timestamp_ms))
                        except asyncio.QueueFull:
                            logger.warning("A track's queue was full, frame dropped.")
                        except asyncio.QueueEmpty:
                            pass # Race condition, safe to ignore
                
            except Exception as e:
                logger.error(f"Error in capture loop: {e}", exc_info=True)
            
            finally:
                if 'cap' in locals() and cap.isOpened():
                    await loop.run_in_executor(None, cap.release)
                logger.info("RTSP stream released. Reconnecting in 5 seconds...")
                await asyncio.sleep(5)


class StreamTrack(VideoStreamTrack):
    """
    Represents a video track for a single WebRTC peer connection.
    This acts as the "Consumer".
    """
    def __init__(self, manager):
        super().__init__()
        self.manager = manager
        self.queue = asyncio.Queue(maxsize=30)
        self._stopped = False

    async def recv(self):
        frame_data, rtsp_timestamp_ms = await self.queue.get()
        
        frame = VideoFrame.from_ndarray(frame_data, format="bgr24")

        # Convert the original millisecond timestamp to a 90kHz PTS value
        time_base = Fraction(1, 90000)
        pts = int(rtsp_timestamp_ms * 90)
        
        frame.pts = pts
        frame.time_base = time_base
        return frame

    async def start(self):
        await self.manager.register(self)

    def stop(self):
        if not self._stopped:
            self._stopped = True
            # Schedule the unregister task, don't await it.
            # This is because aiortc calls stop() synchronously.
            asyncio.create_task(self.manager.unregister(self))


# --- Web Server ---
pcs = set()

async def index(request):
    html_content = """
    <!DOCTYPE html>
    <html>
    <head>
        <title>Bambu X1C Stream</title>
        <style>
            body {
                background-color: #000;
                margin: 0;
                padding: 0;
                overflow: hidden;
                display: flex;
                justify-content: center;
                align-items: center;
                height: 100vh;
            }
            video {
                width: 100%;
                height: 100%;
                object-fit: contain;
            }
        </style>
    </head>
    <body>
        <video id="video" autoplay playsinline muted></video>
        <script src="/client.js"></script>
    </body>
    </html>
    """
    return web.Response(content_type="text/html", text=html_content)

async def javascript(request):
    js_content = """
    let pc;
    const video = document.getElementById('video');

    function createPeerConnection() {
        pc = new RTCPeerConnection({ sdpSemantics: 'unified-plan' });

        pc.addEventListener('track', (evt) => {
            console.log('Track received:', evt.track.kind);
            if (evt.track.kind === 'video') {
                video.srcObject = evt.streams[0];
            }
        });

        pc.addEventListener('connectionstatechange', () => {
            console.log('Connection state:', pc.connectionState);
            if (pc.connectionState === 'failed' || pc.connectionState === 'closed' || pc.connectionState === 'disconnected') {
                console.log('Connection failed, attempting to reconnect in 5 seconds...');
                setTimeout(start, 5000);
            }
        });
    }

    async function start() {
        console.log('Attempting to connect...');
        if(pc) {
            pc.close();
        }
        createPeerConnection();

        try {
            pc.addTransceiver('video', { direction: 'recvonly' });

            const offer = await pc.createOffer();
            await pc.setLocalDescription(offer);

            const response = await fetch('/offer', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ sdp: pc.localDescription.sdp, type: pc.localDescription.type }),
            });

            if (!response.ok) {
                throw new Error(`Server responded with status ${response.status}`);
            }

            const answer = await response.json();
            await pc.setRemoteDescription(answer);
        } catch (e) {
            console.error('Failed to start stream:', e);
            if (pc) {
                pc.close(); // This will trigger the 'connectionstatechange' handler for retry
            }
        }
    }
    start();
    """
    return web.Response(content_type="application/javascript", text=js_content)

async def offer(request):
    params = await request.json()
    offer = RTCSessionDescription(sdp=params["sdp"], type=params["type"])

    pc = RTCPeerConnection()
    pcs.add(pc)
    
    stream_manager = request.app['stream_manager']
    track = StreamTrack(manager=stream_manager)

    @pc.on("connectionstatechange")
    async def on_connectionstatechange():
        logger.info(f"PC connection state is {pc.connectionState}")
        if pc.connectionState in ["failed", "closed", "disconnected"]:
            if pc in pcs:
                pcs.discard(pc)
            # The track's stop() will be called automatically by aiortc
            # upon transport closure, so we don't need to call it here.

    pc.addTrack(track)
    await track.start()

    await pc.setRemoteDescription(offer)
    answer = await pc.createAnswer()
    await pc.setLocalDescription(answer)

    return web.Response(
        content_type="application/json",
        text=json.dumps({"sdp": pc.localDescription.sdp, "type": pc.localDescription.type}),
    )

async def on_shutdown(app):
    coros = [pc.close() for pc in pcs]
    await asyncio.gather(*coros)
    pcs.clear()
    await app['stream_manager'].stop()

# --- Main Application Setup ---
async def start_background_tasks(app):
    app['stream_manager'] = StreamManager(rtsp_url=RTSP_URL)
    await app['stream_manager'].start()

def main():
    app = web.Application()
    app.on_startup.append(start_background_tasks)
    app.on_shutdown.append(on_shutdown)
    app.router.add_get("/", index)
    app.router.add_get("/client.js", javascript)
    app.router.add_post("/offer", offer)
    
    logger.info(f"Starting web server on http://0.0.0.0:{WEB_PORT}")
    web.run_app(app, host="0.0.0.0", port=WEB_PORT)

if __name__ == "__main__":
    main()