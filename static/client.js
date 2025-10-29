let pc;
const video = document.getElementById('video');
const loader = document.getElementById('loader');
const percentage = document.getElementById('percentage');
const pieContainer = document.querySelector('.pie-container');
let progress = 0;
let timer;

function startLoadingAnimation() {
    progress = 0;
    percentage.textContent = '0%';
    pieContainer.style.setProperty('--progress', '0deg');
    loader.style.opacity = 1;
    loader.style.display = 'flex';

    // Simulate loading progress over ~5 seconds
    timer = setInterval(() => {
        progress += 1;
        if (progress <= 100) {
            const angle = progress * 3.6;
            percentage.textContent = Math.round(progress) + '%';
            pieContainer.style.setProperty('--progress', `${angle}deg`);
        } else {
            clearInterval(timer);
        }
    }, 40); // Update every 40ms for a 4-second 100%
}

video.addEventListener('playing', () => {
    console.log('Video is playing.');
    clearInterval(timer); // Stop the timer
    percentage.textContent = '100%';
    pieContainer.style.setProperty('--progress', '360deg');
    // Fade out the loader
    loader.style.opacity = 0;
    setTimeout(() => {
        loader.style.display = 'none';
    }, 500); // Hide after fade out animation
});

function createPeerConnection() {
    pc = new RTCPeerConnection({ sdpSemantics: 'unified-plan' });

    pc.addEventListener('track', (evt) => {
        console.log('Track received:', evt.track.kind);
        if (evt.track.kind === 'video') {
            video.srcObject = evt.streams[0];
            evt.track.onmute = () => {
                console.log('Track muted, no RTP data received for a while.');
            };
            evt.track.onunmute = () => {
                console.log('Track unmuted, RTP data is flowing again.');
            };
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
    startLoadingAnimation();
    
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