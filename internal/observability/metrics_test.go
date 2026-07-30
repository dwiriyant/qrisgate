package observability

import (
	"context"
	"log/slog"
	"testing"
)

func TestNormalizeOTLPEndpoint(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://jaeger:4317", "jaeger:4317"},
		{"http://localhost:4317", "localhost:4317"},
		{"  host:4317  ", "host:4317"},
		{"collector:4317", "collector:4317"},
	}
	for _, tc := range tests {
		if got := NormalizeOTLPEndpoint(tc.in); got != tc.want {
			t.Fatalf("%q => %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewLogger(t *testing.T) {
	log := NewLogger(slog.LevelInfo)
	if log == nil {
		t.Fatal("nil logger")
	}
	ctx := context.Background()
	if !log.Enabled(ctx, slog.LevelInfo) {
		t.Fatal("expected enabled at info")
	}
}

func TestStartMetricsServer_emptyAddr(t *testing.T) {
	// ponytail: no-op when addr blank; just ensure no panic
	StartMetricsServer("", slog.Default())
}
