// Package httpfetch provides bounded, cancellable HTTP downloads.
package httpfetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultTimeout = 30 * time.Second

// Get downloads at most limit bytes. Oversized responses are errors, not truncated data.
func Get(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URL, status code: %d", resp.StatusCode)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("download limit must be positive")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d byte limit", limit)
	}
	return data, nil
}
