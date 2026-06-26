package securego

import (
	"net"
	"net/url"
	"testing"
)

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}

func parseURL(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}

func TestIsValidOutboundURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Allowed
		{"https external", "https://api.openai.com/v1/models", false},
		{"https with port", "https://api.example.com:8443/v1", false},
		{"https with path", "https://api.example.com/v1/models?limit=10", false},

		// Blocked — wrong scheme
		{"http scheme", "http://api.openai.com/v1/models", true},
		{"file scheme", "file:///etc/passwd", true},
		{"data scheme", "data:text/html,<h1>hi</h1>", true},
		{"ftp scheme", "ftp://example.com/file", true},

		// Blocked — userinfo
		{"userinfo", "https://user:pass@api.example.com/v1", true},

		// Blocked — loopback
		{"localhost", "https://localhost/v1/models", true},
		{"localhost with port", "https://localhost:8080/v1", true},
		{"127.0.0.1", "https://127.0.0.1/v1/models", true},
		{"127.0.0.1 with port", "https://127.0.0.1:3000/v1", true},

		// Blocked — private ranges
		{"10.x", "https://10.0.0.1/v1", true},
		{"172.16.x", "https://172.16.0.1/v1", true},
		{"192.168.x", "https://192.168.1.1/v1", true},
		{"169.254.x", "https://169.254.0.1/v1", true},

		// Blocked — empty
		{"empty url", "", true},
		{"no host", "https://", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := IsValidOutboundURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidOutboundURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"public", "8.8.8.8", false},
		{"google dns", "1.1.1.1", false},
		{"loopback", "127.0.0.1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 192.168.x", "192.168.1.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlockedIP(parseIP(tt.ip))
			if got != tt.want {
				t.Errorf("isBlockedIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIsAllowedRedirect(t *testing.T) {
	tests := []struct {
		name        string
		dst         string
		allowed     []string
		want        bool
	}{
		{"allowed", "https://api.example.com/v1", []string{"api.example.com"}, true},
		{"not allowed", "https://evil.com/v1", []string{"api.example.com"}, false},
		{"nil dst", "", []string{"api.example.com"}, false},
		{"nil allowed", "https://api.example.com/v1", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := parseURL(tt.dst)
			got := IsAllowedRedirect(dst, tt.allowed)
			if got != tt.want {
				t.Errorf("IsAllowedRedirect(%s) = %v, want %v", tt.dst, got, tt.want)
			}
		})
	}
}
