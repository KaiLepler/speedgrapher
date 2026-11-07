package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaiLepler/speedgrapher/internal/tools/fog"
)

type failingInput struct{}

func (failingInput) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestRawStdin(t *testing.T) {
	for _, tool := range []string{"fog", "slop", "seo"} {
		t.Run(tool, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if err := Run(context.Background(), "test", []string{"call", tool},
				strings.NewReader("This is ordinary text. Another sentence."), &out, &errOut); err != nil {
				t.Fatal(err)
			}
			if out.Len() == 0 {
				t.Fatal("missing output")
			}
		})
	}
	var params fog.FogParams
	if err := parseArgs(nil, failingInput{}, &params); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("read error discarded: %v", err)
	}
	for _, data := range []string{"", "  ", "{bad json", "[bad json"} {
		if err := parseArgs(nil, strings.NewReader(data), &params); err == nil {
			t.Fatalf("expected input error for %q", data)
		}
	}
}

func TestCallConfiguration(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "speedgrapher.json")
	os.WriteFile(cfg, []byte(`{"tools":{"slop":false}}`), 0600)
	os.WriteFile(filepath.Join(dir, "draft.md"), []byte("A simple sentence."), 0600)
	var out, errOut bytes.Buffer
	if err := Run(context.Background(), "test", []string{"--config", cfg, "call", "fog", `{"path":"draft.md"}`},
		nil, &out, &errOut); err != nil {
		t.Fatalf("external workspace not applied: %v", err)
	}
	err := Run(context.Background(), "test", []string{"--config", cfg, "call", "slop", `{"text":"test"}`},
		nil, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("tool flag ignored: %v", err)
	}
	os.WriteFile(cfg, []byte("{bad"), 0600)
	for _, command := range [][]string{{"mcp", "--config", cfg}, {"--config", cfg, "call", "fog", `{"text":"test"}`}} {
		if err := Run(context.Background(), "test", command, nil, &out, &errOut); err == nil || !strings.Contains(err.Error(), "configuration") {
			t.Fatalf("bad config ignored: %v", err)
		}
	}
}
