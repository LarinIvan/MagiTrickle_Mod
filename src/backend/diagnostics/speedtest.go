package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/showwin/speedtest-go/speedtest"
)

type OoklaServerJSON struct {
	URL     string  `json:"url"`
	Lat     string  `json:"lat"`
	Lon     string  `json:"lon"`
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Sponsor string  `json:"sponsor"`
	ID      string  `json:"id"`
	Host    string  `json:"host"`
	Dist    float64 `json:"distance"`
}

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
	if n > 0 && c.Tracker != nil {
		c.Tracker.Add(n)
	}
	return n, err
}

func (c *CountingReadCloser) Close() error {
	return c.ReadCloser.Close()
}

// RunSpeedtestStream performs a speedtest and streams results via SSE
// It expects w to be an http.ResponseWriter that supports flushing
func RunSpeedtestStream(w http.ResponseWriter, ifaceName string, serverID int, parallelLoss bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var mu sync.Mutex
	isClosed := false
	sendEvent := func(event string, data any) {
		mu.Lock()
		defer mu.Unlock()
		if isClosed {
			return
		}
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\n", event)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
	}

	defer func() {
		mu.Lock()
		isClosed = true
		mu.Unlock()
	}()

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

	var target *speedtest.Server
	if serverID != 0 {
		sendEvent("status", fmt.Sprintf("Finding specified server (ID: %d)...", serverID))
		target, err = speedtestClient.FetchServerByID(strconv.Itoa(serverID))
		if err != nil {
			log.Warn().Int("server_id", serverID).Err(err).Msg("Specified server not found, falling back to auto-select")
		}
	}

	if target == nil {
		sendEvent("status", "Finding best server...")
		serverList, err := speedtestClient.FetchServers()
		if err != nil {
			sendEvent("error", fmt.Sprintf("Failed to fetch servers: %v", err))
			return
		}
		targets, _ := serverList.FindServer([]int{})
		if len(targets) == 0 {
			sendEvent("error", "No servers available")
			return
		}
		target = targets[0]
	}

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
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
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
	sendEvent("stage", "ping")
	tracker.Reset()
	if err := target.PingTest(nil); err != nil {
		sendEvent("error", fmt.Sprintf("Ping failed: %v", err))
		close(doneCh)
		return
	}
	sendEvent("result_ping", float64(target.Latency.Milliseconds()))
	sendEvent("result_jitter", float64(target.Jitter.Milliseconds()))
	time.Sleep(1500 * time.Millisecond)

	// Parallel Loss Logic
	var lossResultCh chan float64

	// If Parallel Loss is enabled, start it NOW (after Ping, concurrently with Download/Upload)
	if parallelLoss {
		lossResultCh = make(chan float64, 1)
		go func() {
			lossResultCh <- measurePacketLoss(target)
		}()
	}

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
	time.Sleep(2500 * time.Millisecond)

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

	// Stop bandwidth monitor
	close(doneCh)
	<-monitorDone
	time.Sleep(2500 * time.Millisecond)

	var lastLoss float64

	if parallelLoss {
		// Parallel Mode: Wait for result or timeout
		sendEvent("status", "Finalizing packet loss...")
		sendEvent("stage", "loss")
		select {
		case res := <-lossResultCh:
			lastLoss = res
		case <-time.After(5 * time.Second):
			// Timeout
			log.Warn().Msg("Parallel packet loss test timed out")
			lastLoss = -1
		}
	} else {
		// Sequential Mode: Run it now
		sendEvent("status", "Packet loss test...")
		sendEvent("stage", "loss")
		lastLoss = measurePacketLoss(target)
	}

	sendEvent("result_packet_loss", lastLoss)

	// Finish
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
		ForceAttemptHTTP2:     false,
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

// searchServersViaAPI queries the Ookla API for servers matching the search string
func searchServersViaAPI(client *http.Client, query string) ([]*speedtest.Server, error) {
	url := fmt.Sprintf("https://www.speedtest.net/api/js/servers?search=%s", neturl.QueryEscape(query))
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status: %s", resp.Status)
	}

	var jsonServers []OoklaServerJSON
	if err := json.NewDecoder(resp.Body).Decode(&jsonServers); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	var servers []*speedtest.Server
	for _, js := range jsonServers {
		server := &speedtest.Server{
			ID:       js.ID,
			Name:     js.Name,
			Country:  js.Country,
			Sponsor:  js.Sponsor,
			Host:     js.Host,
			Distance: js.Dist,
			Lat:      js.Lat,
			Lon:      js.Lon,
		}
		servers = append(servers, server)
	}
	return servers, nil
}

// GetServers returns a list of available speedtest servers, optionally filtered by search string
func GetServers(ifaceName string, search string) ([]*speedtest.Server, error) {
	client := createMonitoringClient(ifaceName, nil)
	speedtestClient := speedtest.New(speedtest.WithDoer(client))

	if search != "" {
		log.Info().Str("query", search).Msg("Searching servers via Ookla API")

		var results []*speedtest.Server
		var err error

		// Try via interface (VPN) 3 times
		for i := 0; i < 3; i++ {
			results, err = searchServersViaAPI(client, search)
			if err == nil {
				break
			}
			log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to search servers via API (interface), retrying...")
			time.Sleep(1 * time.Second)
		}

		if err != nil {
			log.Warn().Err(err).Msg("Failed to search servers via API (interface) after 3 attempts, trying default route")
			// Fallback
			defaultClient := &http.Client{Timeout: 10 * time.Second}
			results, err = searchServersViaAPI(defaultClient, search)
			if err != nil {
				log.Error().Err(err).Msg("Failed to search servers via API (both)")
				return nil, err
			}
			log.Info().Msg("Search successful via default route")
		}
		log.Info().Int("count", len(results)).Msg("API search completed")
		return results, nil
	}

	// Helper to fetch server list with fallback
	fetchList := func(c *http.Client) (speedtest.Servers, error) {
		st := speedtest.New(speedtest.WithDoer(c))
		return st.FetchServers()
	}

	// Default behavior: fetch nearest servers
	var user *speedtest.User
	var err error

	// 1. Fetch User Info (Try 3 times via interface)
	for i := 0; i < 3; i++ {
		user, err = speedtestClient.FetchUserInfo()
		if err == nil {
			break
		}
		log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to fetch user info via interface, retrying...")
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch user info for servers list via interface after 3 attempts, trying default")
		// Try default client for user info as a check
		defaultClient := &http.Client{Timeout: 10 * time.Second}
		stDefault := speedtest.New(speedtest.WithDoer(defaultClient))
		if u, e := stDefault.FetchUserInfo(); e == nil {
			user = u
			log.Info().Msg("Fetched user info via default route")
		}
	} else {
		log.Debug().Str("ip", user.IP).Msg("User info fetched")
	}

	// 2. Fetch Server List (Try 3 times via interface)
	var serverList speedtest.Servers
	for i := 0; i < 3; i++ {
		serverList, err = fetchList(client)
		if err == nil {
			break
		}
		log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to fetch servers via interface, retrying...")
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch servers via interface after 3 attempts, trying default route")
		// Fallback to default client (no interface binding)
		defaultClient := &http.Client{Timeout: 10 * time.Second}
		serverList, err = fetchList(defaultClient)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch servers (both interface and default): %w", err)
		}
		log.Info().Msg("Fetched servers via default route")
	}

	// FindServer filters and sorts by latency (via ping)
	allSorted := serverList
	log.Debug().Int("sorted_count", len(allSorted)).Msg("Fetched nearest servers (raw list)")

	// Return top 50 nearest
	if len(allSorted) > 50 {
		return allSorted[:50], nil
	}
	return allSorted, nil
}
