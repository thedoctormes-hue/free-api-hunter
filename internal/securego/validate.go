// Package securego provides security helpers for SSRF prevention,
// URL validation, and input sanitization.
package securego

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// IsValidOutboundURL validates that a URL is safe for outbound HTTP requests.
// It blocks:
//   - non-HTTPS schemes (http, file, data, etc.)
//   - userinfo (user:pass@host)
//   - loopback addresses (127.0.0.0/8, ::1, localhost)
//   - private/internal IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, etc.)
//   - link-local addresses (169.254.0.0/16, fe80::/10)
//
// Returns the parsed URL or an error describing why the URL was rejected.
func IsValidOutboundURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Only HTTPS allowed
	if u.Scheme != "https" {
		return nil, fmt.Errorf("scheme %q not allowed, only https", u.Scheme)
	}

	// Block userinfo (ssrf: http://evil@internal-host/)
	if u.User != nil {
		return nil, fmt.Errorf("userinfo in URL not allowed")
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("empty host")
	}

	// Check for DNS rebinding: "localhost" literal
	if strings.EqualFold(host, "localhost") {
		return nil, fmt.Errorf("loopback host blocked")
	}

	// Resolve and check IP ranges (defense against DNS rebinding)
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed: %w", err)
	}

	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("IP %s is blocked (private/loopback/link-local)", ip)
		}
	}

	return u, nil
}

// IsAllowedRedirect checks whether a redirect destination host is in the
// allowed hosts list. Returns false if the URL cannot be parsed.
func IsAllowedRedirect(dst *url.URL, allowedHosts []string) bool {
	if dst == nil || allowedHosts == nil {
		return false
	}
	for _, h := range allowedHosts {
		if strings.EqualFold(dst.Host, h) {
			return true
		}
	}
	return false
}

// isBlockedIP checks if an IP is in a private, loopback, or link-local range
func isBlockedIP(ip net.IP) bool {
	blocked := []*net.IPNet{
		{IP: net.ParseIP("127.0.0.0").To4(), Mask: net.CIDRMask(8, 32)},       // 127.0.0.0/8
		{IP: net.ParseIP("10.0.0.0").To4(), Mask: net.CIDRMask(8, 32)},          // 10.0.0.0/8
		{IP: net.ParseIP("172.16.0.0").To4(), Mask: net.CIDRMask(12, 32)},      // 172.16.0.0/12
		{IP: net.ParseIP("192.168.0.0").To4(), Mask: net.CIDRMask(16, 32)},     // 192.168.0.0/16
		{IP: net.ParseIP("169.254.0.0").To4(), Mask: net.CIDRMask(16, 32)},     // 169.254.0.0/16
		{IP: net.ParseIP("100.64.0.0").To4(), Mask: net.CIDRMask(10, 32)},      // 100.64.0.0/10
	}

	for _, cidr := range blocked {
		if cidr.Contains(ip) {
			return true
		}
	}

	// IPv6 loopback and link-local
	if ip.To4() == nil {
		if ip.Equal(net.ParseIP("::1")) {
			return true
		}
		if ip.IsLinkLocalUnicast() {
			return true
		}
	}

	return false
}
