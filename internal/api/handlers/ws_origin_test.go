package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebsocketOriginPolicy(t *testing.T) {
	policy := websocketOriginPolicy("https://agent.getvicinify.com, https://admin.example.com:8443/")
	for _, test := range []struct {
		origin, host string
		allowed      bool
	}{
		{"https://agent.getvicinify.com", "api-agent.getvicinify.com", true},
		{"https://AGENT.getvicinify.com:443", "api-agent.getvicinify.com", true},
		{"https://admin.example.com:8443", "api.example.com", true},
		{"https://admin.example.com", "api.example.com", false},
		{"http://agent.getvicinify.com", "api-agent.getvicinify.com", false},
		{"https://agent.getvicinify.com.evil.test", "api-agent.getvicinify.com", false},
		{"https://agent.getvicinify.com@evil.test", "api-agent.getvicinify.com", false},
		{"https://agent.getvicinify.com/extra", "api-agent.getvicinify.com", false},
		{"https://agent.getvicinify.com?q=1", "api-agent.getvicinify.com", false},
		{"null", "api-agent.getvicinify.com", false},
		{"http://localhost:3000", "localhost:8080", true},
		{"", "api-agent.getvicinify.com", true},
	} {
		r := httptest.NewRequest("GET", "http://"+test.host+"/api/v1/ws/test", nil)
		r.Header.Set("Origin", test.origin)
		r.Header.Set("X-Forwarded-Host", "agent.getvicinify.com")
		if got := policy(r); got != test.allowed {
			t.Errorf("%s with host %s: got %v", test.origin, test.host, got)
		}
	}
	r := httptest.NewRequest("GET", "http://api.example.com/ws", nil)
	r.Header.Add("Origin", "https://agent.getvicinify.com")
	r.Header.Add("Origin", "https://evil.test")
	if policy(r) {
		t.Fatal("accepted multiple origins")
	}
	if websocketOriginPolicy("")(httptest.NewRequest("GET", "http://example.com", nil)) != true {
		t.Fatal("nonbrowser clients rejected")
	}
	r.Header = make(http.Header)
	r.Header.Set("Origin", "https://agent.getvicinify.com")
	if websocketOriginPolicy("")(r) {
		t.Fatal("cross-domain allowed without configuration")
	}
}

func TestCrossDomainWebsocketHandshake(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: websocketOriginPolicy("https://agent.getvicinify.com")}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			conn.Close()
		}
	}))
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http")
	for _, test := range []struct {
		origin  string
		allowed bool
	}{
		{"https://agent.getvicinify.com", true},
		{"https://untrusted.example", false},
	} {
		conn, response, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Origin": []string{test.origin}})
		if test.allowed {
			if err != nil {
				t.Fatalf("configured origin handshake failed: %v", err)
			}
			conn.Close()
		} else {
			if err == nil {
				conn.Close()
				t.Fatal("untrusted browser connected")
			}
			if response == nil || response.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 403: %+v", response)
			}
		}
	}
}
