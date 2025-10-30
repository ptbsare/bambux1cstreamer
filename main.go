package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

// --- Configuration ---
var (
	rtspURL       = os.Getenv("RTSP_URL")
	webPort       = "33003" // Use a different port than the python version
	rtspProxyPort = "8554"
	webrtcPortMin uint16
	webrtcPortMax uint16
	webrtcAPI     *webrtc.API
)

// StreamManager manages a single RTSP connection and broadcasts H264 frames to multiple listeners.
type StreamManager struct {
	lock            sync.RWMutex
	webrtcListeners map[*webrtc.TrackLocalStaticRTP]struct{}
	rtspURL         *base.URL
	format          *format.H264
	media           *description.Media
	rtspStream      *gortsplib.ServerStream
	rtspServer      *gortsplib.Server
}

var streamManager *StreamManager

// NewStreamManager creates and starts a new StreamManager.
func NewStreamManager(u *base.URL) *StreamManager {
	m := &StreamManager{
		webrtcListeners: make(map[*webrtc.TrackLocalStaticRTP]struct{}),
		rtspURL:         u,
	}
	go m.run()
	return m
}

// run is the main loop for connecting to RTSP and reading frames.
func (m *StreamManager) run() {
	for {
		log.Printf("Connecting to RTSP stream: %s", m.rtspURL)

		c := &gortsplib.Client{
			Scheme: m.rtspURL.Scheme,
			Host:   m.rtspURL.Host,
		}

		// Skip TLS certificate verification for rtsps streams, as printers use self-signed certs.
		if c.Scheme == "rtsps" {
			c.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		}

		err := c.Start()
		if err != nil {
			log.Printf("Failed to connect to RTSP: %v. Retrying in 5 seconds...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		desc, _, err := c.Describe(m.rtspURL)
		if err != nil {
			log.Printf("Failed to describe RTSP stream: %v. Retrying...", err)
			c.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		var h264Format *format.H264
		h264Media := desc.FindFormat(&h264Format)
		if h264Media == nil {
			log.Printf("H264 track not found in RTSP stream. Retrying...")
			c.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("Found H264 format: %+v", h264Format)

		err = c.SetupAll(desc.BaseURL, desc.Medias)
		if err != nil {
			log.Printf("Failed to setup medias: %v. Retrying...", err)
			c.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		m.lock.Lock()
		m.format = h264Format
		m.media = h264Media

		// create the RTSP stream
		m.rtspStream = &gortsplib.ServerStream{
			Server: m.rtspServer,
			Desc: &description.Session{
				Medias: []*description.Media{m.media},
			},
		}
		err = m.rtspStream.Initialize()
		if err != nil {
			log.Printf("Error initializing RTSP stream: %v", err)
			c.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		m.lock.Unlock()

		c.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
			// We only care about the H264 media stream
			if medi != h264Media {
				return
			}
			// log.Printf("RTP Packet: PT=%d, Seq=%d, TS=%d, Len=%d", pkt.PayloadType, pkt.SequenceNumber, pkt.Timestamp, len(pkt.Payload))

			m.lock.RLock()
			defer m.lock.RUnlock()

			// Forward packet to all WebRTC listeners
			for track := range m.webrtcListeners {
				err := track.WriteRTP(pkt)
				if err != nil {
					// log.Printf("Error writing RTP to track: %v", err)
				}
			}

			// Forward packet to the RTSP stream. It will then distribute it to all listeners.
			if m.rtspStream != nil {
				m.rtspStream.WritePacketRTP(medi, pkt)
			}
		})

		_, err = c.Play(nil)
		if err != nil {
			log.Printf("Failed to start playing: %v. Retrying...", err)
			c.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		log.Println("RTSP stream connected and playing successfully.")
		err = c.Wait()
		log.Printf("RTSP connection lost: %v. Reconnecting...", err)

		m.lock.Lock()
		m.format = nil
		m.media = nil
		m.rtspStream = nil
		m.lock.Unlock()
	}
}

// AddWebRTCListener registers a new WebRTC track to receive H264 frames.
func (m *StreamManager) AddWebRTCListener(track *webrtc.TrackLocalStaticRTP) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.webrtcListeners[track] = struct{}{}
	log.Printf("WebRTC listener added. Total listeners: %d", len(m.webrtcListeners))
}

// RemoveWebRTCListener unregisters a WebRTC track.
func (m *StreamManager) RemoveWebRTCListener(track *webrtc.TrackLocalStaticRTP) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.webrtcListeners, track)
	log.Printf("WebRTC listener removed. Total listeners: %d", len(m.webrtcListeners))
}

// serverHandler implements the gortsplib.ServerHandler interface.
type serverHandler struct{}

func (h *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("describe request for path %q", ctx.Path)

	if ctx.Path != "/stream" {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, fmt.Errorf("path not found")
	}

	// Wait until the stream is ready
	for i := 0; i < 100; i++ { // Wait up to 10 seconds
		streamManager.lock.RLock()
		ready := streamManager.rtspStream != nil
		streamManager.lock.RUnlock()
		if ready {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	streamManager.lock.RLock()
	defer streamManager.lock.RUnlock()

	if streamManager.rtspStream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, fmt.Errorf("stream not ready")
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, streamManager.rtspStream, nil
}

func (h *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("setup request for path %q", ctx.Path)
	return &base.Response{StatusCode: base.StatusOK}, streamManager.rtspStream, nil
}

func (h *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("play request for path %q", ctx.Path)
	return &base.Response{StatusCode: base.StatusOK}, nil
}

func (h *serverHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("session closed")
}

func main() {
	if rtspURL == "" {
		log.Fatal("RTSP_URL environment variable not set. Please set it to your camera's RTSP stream URL.")
	}
	if p := os.Getenv("WEB_PORT"); p != "" {
		webPort = p
	}
	if p := os.Getenv("RTSP_PROXY_PORT"); p != "" {
		rtspProxyPort = p
	}
	if minPortStr := os.Getenv("WEBRTC_UDP_PORT_MIN"); minPortStr != "" {
		minPort, err := strconv.ParseUint(minPortStr, 10, 16)
		if err == nil {
			webrtcPortMin = uint16(minPort)
		} else {
			log.Printf("Invalid WEBRTC_UDP_PORT_MIN value: %v", err)
		}
	}
	if maxPortStr := os.Getenv("WEBRTC_UDP_PORT_MAX"); maxPortStr != "" {
		maxPort, err := strconv.ParseUint(maxPortStr, 10, 16)
		if err == nil {
			webrtcPortMax = uint16(maxPort)
		} else {
			log.Printf("Invalid WEBRTC_UDP_PORT_MAX value: %v", err)
		}
	}

	// Create a SettingEngine and configure the port range
	settingEngine := webrtc.SettingEngine{}

	// Register the H264 codec
	mediaEngine := &webrtc.MediaEngine{}
	err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Fatalf("Failed to register H264 codec: %v", err)
	}

	// Force IPv6
	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP6,
		webrtc.NetworkTypeTCP6,
		webrtc.NetworkTypeUDP4,
		webrtc.NetworkTypeTCP4,
	})

	if webrtcPortMin > 0 && webrtcPortMax > 0 {
		if webrtcPortMin > webrtcPortMax {
			log.Fatal("WEBRTC_UDP_PORT_MIN cannot be greater than WEBRTC_UDP_PORT_MAX")
		}
		err := settingEngine.SetEphemeralUDPPortRange(webrtcPortMin, webrtcPortMax)
		if err != nil {
			log.Fatalf("Failed to set WebRTC port range: %v", err)
		}
		log.Printf("WebRTC UDP port range set to %d-%d", webrtcPortMin, webrtcPortMax)
	} else {
		log.Printf("WebRTC UDP port range is not configured, using random ports")
	}

	webrtcAPI = webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithSettingEngine(settingEngine))

	u, err := base.ParseURL(rtspURL)
	if err != nil {
		log.Fatalf("Invalid RTSP URL: %v", err)
	}

	// Initialize the singleton StreamManager
	streamManager = NewStreamManager(u)

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/offer", handleOffer)

	log.Printf("Starting web server on http://0.0.0.0:%s", webPort)
	go func() {
		if err := http.ListenAndServe(":"+webPort, nil); err != nil {
			log.Fatalf("Failed to start web server: %v", err)
		}
	}()

	// Create a new gortsplib server
	s := &gortsplib.Server{
		Handler:     &serverHandler{},
		RTSPAddress: ":" + rtspProxyPort,
	}
	streamManager.rtspServer = s

	log.Printf("Starting RTSP proxy on rtsp://0.0.0.0:%s", rtspProxyPort)
	// start server and wait until a fatal error
	panic(s.StartAndWait())
}

func handleOffer(w http.ResponseWriter, r *http.Request) {
	// Decode the offer from the request body
	var offer webrtc.SessionDescription
	err := json.NewDecoder(r.Body).Decode(&offer)
	if err != nil {
		log.Printf("Error decoding offer: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// We need to wait until the RTSP stream is connected and we have the format info
	for i := 0; i < 100; i++ { // Wait up to 10 seconds
		streamManager.lock.RLock()
		ready := streamManager.format != nil && streamManager.media != nil
		streamManager.lock.RUnlock()
		if ready {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	streamManager.lock.RLock()
	ready := streamManager.format != nil && streamManager.media != nil
	streamManager.lock.RUnlock()
	if !ready {
		log.Println("Error: RTSP stream not ready, format/media is nil")
		http.Error(w, "Stream not ready", http.StatusInternalServerError)
		return
	}

	// Create a new PeerConnection
	peerConnection, err := webrtcAPI.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Printf("Error creating PeerConnection: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Create a video track using the codec from the RTSP stream
	codec := webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH264,
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 96, // Standard H264 payload type
	}
	if streamManager.format != nil {
		// Convert the FMTP map to a string
		var fmtpLine string
		for k, v := range streamManager.format.FMTP() {
			fmtpLine += k + "=" + v + ";"
		}
		// Remove the trailing semicolon
		if len(fmtpLine) > 0 {
			fmtpLine = fmtpLine[:len(fmtpLine)-1]
		}
		codec.SDPFmtpLine = fmtpLine
		log.Printf("Using SDP Fmtp Line: %s", fmtpLine)
	}

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(codec.RTPCodecCapability, "video", "pion")
	if err != nil {
		log.Printf("Error creating video track: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rtpSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		log.Printf("Error adding track: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be enabled.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// Add the track to the stream manager
	streamManager.AddWebRTCListener(videoTrack)

	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("Peer Connection State has changed: %s", s.String())
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateDisconnected || s == webrtc.PeerConnectionStateClosed {
			streamManager.RemoveWebRTCListener(videoTrack)
			peerConnection.Close()
		}
	})

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		log.Printf("Error setting remote description: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Printf("Error creating answer: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		log.Printf("Error setting local description: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// This is less efficient, but simpler.
	<-gatherComplete

	// Get the final answer with all ICE candidates
	finalAnswer := peerConnection.LocalDescription()

	// Send the answer back to the client
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(finalAnswer)
	if err != nil {
		log.Printf("Error encoding answer: %v", err)
	}
}