package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/config"
)

func TestNewHTTPServerSetsHardenedTimeouts(t *testing.T) {
	srv := NewHTTPServer(config.Config{ListenAddr: ":8080"}, http.NotFoundHandler())

	if srv.Addr != ":8080" {
		t.Fatalf("expected addr :8080, got %q", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("expected ReadHeaderTimeout 5s, got %v", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 15*time.Second {
		t.Fatalf("expected ReadTimeout 15s, got %v", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 60*time.Second {
		t.Fatalf("expected WriteTimeout 60s, got %v", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Fatalf("expected IdleTimeout 60s, got %v", srv.IdleTimeout)
	}
	if srv.Handler == nil {
		t.Fatal("expected handler to be wired")
	}
}
