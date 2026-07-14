package tests

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"free-api-hunter/internal/api"
	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/storage"
)

// TestE2EServer boots the real HTTP server on a local port and exercises the
// full request stack (real socket, real http.Client) end-to-end.
func TestE2EServer(t *testing.T) {
	os.Setenv("FREE_API_HUNTER_API_KEY", "test-key")
	api.SetAPIKeys([]string{"test-key"})

	dir := t.TempDir()
	storage.DataDir = dir
	if err := database.Init(dir); err != nil {
		t.Fatalf("database init: %v", err)
	}

	// Seed one provider so /api/v1/providers returns real data.
	prov := &models.Provider{
		Name:   "Groq",
		URL:    "https://groq.com",
		Source: "e2e",
	}
	if err := storage.SaveProviders([]*models.Provider{prov}, ""); err != nil {
		t.Fatalf("save providers: %v", err)
	}

	// Grab a free port, release it, and let the server bind it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s := api.NewServerWithDir(addr, dir)
	go func() { _ = s.ListenAndServe() }()

	base := fmt.Sprintf("http://%s", addr)
	client := &http.Client{Timeout: 5 * time.Second}

	// Wait for the server to accept connections.
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server did not become ready in time")
	}

	// Public read endpoints must respond 200 through the real socket.
	for _, path := range []string{
		"/health",
		"/api/v1/providers",
		"/api/v1/stats",
		"/api/v1/findings",
		"/api/v1/scan-history",
		"/",
	} {
		if code := getStatus(t, client, base+path); code != 200 {
			t.Errorf("GET %s status = %d, want 200", path, code)
		}
	}

	// /api/v1/providers must contain the seeded provider.
	resp, err := client.Get(base + "/api/v1/providers")
	if err != nil {
		t.Fatalf("GET providers: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Groq") {
		t.Errorf("/api/v1/providers body missing seeded provider: %s", string(body))
	}

	// Protected endpoint without key must be rejected.
	req, _ := http.NewRequest("POST", base+"/api/v1/scan", nil)
	noKeyResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST scan without key: %v", err)
	}
	if noKeyResp.StatusCode == 200 || noKeyResp.StatusCode == 202 {
		t.Errorf("scan without key status = %d, want rejection", noKeyResp.StatusCode)
	}
	noKeyResp.Body.Close()

	// Protected endpoint with valid key must accept (202 = scan started).
	reqKey, _ := http.NewRequest("POST", base+"/api/v1/scan", nil)
	reqKey.Header.Set("X-API-Key", "test-key")
	keyResp, err := client.Do(reqKey)
	if err != nil {
		t.Fatalf("POST scan with key: %v", err)
	}
	if keyResp.StatusCode != 202 {
		t.Errorf("scan with key status = %d, want 202", keyResp.StatusCode)
	}
	keyResp.Body.Close()
}

func getStatus(t *testing.T, client *http.Client, url string) int {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
