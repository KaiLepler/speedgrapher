package server_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KaiLepler/speedgrapher/internal/config"
	"github.com/KaiLepler/speedgrapher/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConfiguredToolsAndWorkspace(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "draft.md"), []byte("A short sentence."), 0600); err != nil {
		t.Fatal(err)
	}
	srv := server.New("test", server.WithWorkspaceDir(ws), server.WithToolsConfig(config.ToolsConfig{Fog: true}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "fog" {
		t.Fatalf("configured tools: %v %v", tools, err)
	}
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "fog", Arguments: map[string]any{"path": "draft.md"}})
	if err != nil || result.IsError {
		t.Fatalf("workspace not applied: %v %v", result, err)
	}
}
