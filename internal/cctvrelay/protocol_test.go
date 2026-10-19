package cctvrelay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGatewayOnlyAcceptsMediaOperations(t *testing.T) {
	for _, request := range []Request{
		{Method: "POST", Path: "/api/exit"}, {Method: "GET", Path: "/api/config"},
		{Method: "GET", Path: "/api/streams"}, {Method: "GET", Path: "http://elsewhere/api/streams"},
		{Method: "PATCH", Path: "/api/streams", Query: url.Values{"name": {"cam_A"}, "src": {"exec:cat /etc/passwd"}}.Encode()},
		{Method: "PATCH", Path: "/api/streams", Query: url.Values{"name": {"cam_A"}, "src": {"rtsp://camera/live#input=exec"}}.Encode()},
	} {
		if ValidRequest(request) {
			t.Fatalf("operasi terlarang diterima: %s %s", request.Method, request.Path)
		}
	}
	if !ValidRequest(Request{Method: "PATCH", Path: "/api/streams", Query: url.Values{"name": {"cam_A"}, "src": {"rtsp://user:pass@camera/live"}}.Encode()}) {
		t.Fatal("registrasi RTSP ditolak")
	}
}

func TestGatewayNeverReturnsSecretErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		io.WriteString(w, "rtsp://admin:password@camera/live")
	}))
	defer server.Close()
	response := Execute(context.Background(), server.Client(), server.URL, Request{ID: "test", Method: "GET", Path: "/api/stream.m3u8", Query: "src=cam_A"})
	if response.Status != 500 || len(response.Body) != 0 {
		t.Fatal("galat sumber bocor ke cloud")
	}
}

func TestGatewayRequiresTLSAndLocalRelay(t *testing.T) {
	for _, cloud := range []string{"http://pos.example.com", "https://user:pass@pos.example.com", "https://pos.example.com?token=secret"} {
		client := Client{CloudURL: cloud, Token: strings.Repeat("a", 64), RelayURL: "http://127.0.0.1:1984"}
		if client.Validate() == nil {
			t.Fatalf("cloud tidak aman diterima: %s", cloud)
		}
	}
	client := Client{CloudURL: "https://pos.example.com", Token: strings.Repeat("a", 64), RelayURL: "http://camera:1984"}
	if client.Validate() == nil {
		t.Fatal("relay non-lokal diterima")
	}
}
