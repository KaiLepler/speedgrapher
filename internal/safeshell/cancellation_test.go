package safeshell_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/KaiLepler/speedgrapher/internal/safeshell"
)

func TestCallerCancellationKillsDescendants(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process groups")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := safeshell.Execute(ctx, "sh", "-c", "sleep 30 & wait")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("descendant kept output pipes open after caller cancellation")
	}
}
