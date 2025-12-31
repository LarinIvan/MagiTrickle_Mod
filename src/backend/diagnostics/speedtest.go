package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/showwin/speedtest-go/speedtest"
)

type SpeedtestResult struct {
	ServerID      string  `json:"server_id"`
	ServerName    string  `json:"server_name"`
	ServerCountry string  `json:"server_country"`
	LatencyMs     float64 `json:"latency_ms"`
	DownloadMbps  float64 `json:"download_mbps"`
	UploadMbps    float64 `json:"upload_mbps"`
	Interface     string  `json:"interface"`
}

// SpeedTracker tracks bytes transferred
type SpeedTracker struct {
	Bytes int64
}

func (st *SpeedTracker) Add(n int) {
	atomic.AddInt64(&st.Bytes, int64(n))
}

func (st *SpeedTracker) Reset() {
	atomic.StoreInt64(&st.Bytes, 0)
}

func (st *SpeedTracker) GetAndReset() int64 {
	return atomic.SwapInt64(&st.Bytes, 0)
}

// MonitoringTransport wraps http.RoundTripper to count bytes
type MonitoringTransport struct {
	Transport http.RoundTripper
	Tracker   *SpeedTracker
}

func (m *MonitoringTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Wrap Request Body (Upload)
	if req.Body != nil {
		req.Body = &CountingReadCloser{
			ReadCloser: req.Body,
			Tracker:    m.Tracker,
		}
	}

	resp, err := m.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Wrap Response Body (Download)
	if resp.Body != nil {
		resp.Body = &CountingReadCloser{
			ReadCloser: resp.Body,
			Tracker:    m.Tracker,
		}
	}

	return resp, nil
}

type CountingReadCloser struct {
	io.ReadCloser
	Tracker *SpeedTracker
}

func (c *CountingReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if n > 0 {
		c.Tracker.Add(n)
	}
	return n, err
}

func (c *CountingReadCloser) Close() error {
	return c.ReadCloser.Close()
}

// RunSpeedtestStream performs a speedtest and streams results via SSE
// It expects w to be an http.ResponseWriter that supports flushing
func RunSpeedtestStream(w http.ResponseWriter, ifaceName string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sendEvent := func(event string, data any) {
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\n", event)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
	}

	log.Info().Str("interface", ifaceName).Msg("Starting streaming speedtest")

	tracker := &SpeedTracker{}
	client := createMonitoringClient(ifaceName, tracker)
	speedtestClient := speedtest.New(speedtest.WithDoer(client))

	// Fetch Servers
	// Fetch User Info (ISP/IP)
	sendEvent("status", "Finding best server...")
	user, err := speedtestClient.FetchUserInfo()
	if err != nil {
		// Log warning but don't fail, we can proceed without user info
		log.Warn().Err(err).Msg("Failed to fetch user info")
	} else {
		sendEvent("client_info", map[string]string{
			"ip":  user.IP,
			"isp": user.Isp,
		})
	}

	serverList, err := speedtestClient.FetchServers()
	if err != nil {
		sendEvent("error", fmt.Sprintf("Failed to fetch servers: %v", err))
		return
	}

	targets, err := serverList.FindServer([]int{})
	if err != nil || len(targets) == 0 {
		sendEvent("error", fmt.Sprintf("Failed to find target server: %v", err))
		return
	}
	target := targets[0]
	sendEvent("server_info", map[string]string{
		"name":    target.Name,
		"country": target.Country,
		"sponsor": target.Sponsor,
		"id":      target.ID,
	})

	// Monitor Goroutine
	updateInterval := 100 * time.Millisecond
	ticker := time.NewTicker(updateInterval)
	doneCh := make(chan bool)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-doneCh:
				return
			case <-ticker.C:
				bytes := tracker.GetAndReset()
				// Formula: (Bytes * 8 bits) / (Interval Seconds * 1,000,000 for Mbps)
				// This is dynamic and depends on updateInterval.
				seconds := updateInterval.Seconds()
				mbps := (float64(bytes) * 8) / (seconds * 1000000)

				if mbps > 0 {
					sendEvent("speed", map[string]float64{"mbps": mbps})
				}
			}
		}
	}()

	// Ping
	sendEvent("status", "Ping test...")
	sendEvent("stage", "ping") // frontend expects 'stage' to switch UI mode
	tracker.Reset()
	if err := target.PingTest(nil); err != nil {
		sendEvent("error", fmt.Sprintf("Ping failed: %v", err))
		close(doneCh)
		return
	}
	sendEvent("result_ping", float64(target.Latency.Milliseconds()))

	// Download
	sendEvent("status", "Download test...")
	sendEvent("stage", "download")
	tracker.Reset()
	if err := target.DownloadTest(); err != nil {
		sendEvent("error", fmt.Sprintf("Download failed: %v", err))
		close(doneCh)
		return
	}
	dlMbps := float64(target.DLSpeed) * 8 / 1000000
	sendEvent("result_download", dlMbps)

	// Upload
	sendEvent("status", "Upload test...")
	sendEvent("stage", "upload")
	tracker.Reset()
	if err := target.UploadTest(); err != nil {
		sendEvent("error", fmt.Sprintf("Upload failed: %v", err))
		close(doneCh)
		return
	}
	ulMbps := float64(target.ULSpeed) * 8 / 1000000
	sendEvent("result_upload", ulMbps)

	// Finish
	close(doneCh)
	sendEvent("done", "Test complete")
}

func createMonitoringClient(ifaceName string, tracker *SpeedTracker) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			if ifaceName == "" {
				return nil
			}
			return c.Control(func(fd uintptr) {
				bindToDevice(fd, ifaceName)
			})
		},
	}

	baseTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: &MonitoringTransport{
			Transport: baseTransport,
			Tracker:   tracker,
		},
	}
}
