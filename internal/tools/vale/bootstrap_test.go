package vale

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDownloadVale(t *testing.T) {
	data := []byte("test archive")
	hash := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		w.Write(data)
	}))
	defer srv.Close()
	if _, err := downloadVale(context.Background(), srv.Client(), srv.URL, hex.EncodeToString(hash[:])); err != nil {
		t.Fatal(err)
	}
	if _, err := downloadVale(context.Background(), srv.Client(), srv.URL, "bad"); err == nil {
		t.Fatal("expected checksum error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := downloadVale(ctx, srv.Client(), srv.URL+"/slow", "bad"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout: %v", err)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestWriteExecutablePreservesOnFailure(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "vale")
	os.WriteFile(dest, []byte("original"), 0700)
	if err := writeExecutable(brokenReader{}, dest); err == nil {
		t.Fatal("expected failure")
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "original" {
		t.Fatal("replaced working binary on failed download")
	}
	if err := writeExecutable(strings.NewReader("replacement"), dest); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(dest)
	if string(data) != "replacement" {
		t.Fatal("replacement missing")
	}
}

func TestBootstrapCacheAndCancelledSync(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	fake := createFakeValeBinary(t)
	// A valid PATH binary is reused without a download.
	dir := t.TempDir()
	binary := filepath.Join(dir, "vale")
	data, _ := os.ReadFile(fake)
	os.WriteFile(binary, data, 0700)
	t.Setenv("PATH", dir)
	if got, err := bootstrapValeContext(context.Background(), ""); err != nil || got != binary {
		t.Fatalf("PATH cache reuse: %q %v", got, err)
	}
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, ".vale.ini"), []byte("StylesPath = styles\n"), 0600)
	os.WriteFile(binary, []byte("#!/bin/sh\n/bin/sleep 30\n"), 0700)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := setupValeConfigContext(ctx, binary, ws); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected sync cancellation: %v", err)
	}
}
