package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendPostsMessage(t *testing.T) {
	var method, body string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		method, body = r.Method, string(content)
	}))
	defer server.Close()

	if err := Send(context.Background(), server.Client(), server.URL, "instance launched"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if method != http.MethodPost || body != "instance launched" {
		t.Errorf("server received %s %q, want POST %q", method, body, "instance launched")
	}
}

func TestSendFailsOnErrorStatus(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := Send(context.Background(), server.Client(), server.URL, "instance launched"); err == nil {
		t.Fatal("Send() error = nil, want an error")
	}
}
