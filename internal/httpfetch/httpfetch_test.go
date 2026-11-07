package httpfetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetBoundsAndCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			<-r.Context().Done()
		case "/error":
			w.WriteHeader(http.StatusNotFound)
		case "/interrupted":
			w.Header().Set("Content-Length", "100")
			w.Write([]byte("short"))
		default:
			w.Write([]byte("12345"))
		}
	}))
	defer srv.Close()
	if data, err := Get(context.Background(), srv.Client(), srv.URL, 5); err != nil || string(data) != "12345" {
		t.Fatalf("exact limit: %q %v", data, err)
	}
	if _, err := Get(context.Background(), srv.Client(), srv.URL, 4); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized response: %v", err)
	}
	if _, err := Get(context.Background(), srv.Client(), srv.URL+"/error", 5); err == nil {
		t.Fatal("expected status error")
	}
	if _, err := Get(context.Background(), srv.Client(), srv.URL+"/interrupted", 100); err == nil {
		t.Fatal("expected interrupted response error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Get(ctx, srv.Client(), srv.URL+"/slow", 5); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
}
