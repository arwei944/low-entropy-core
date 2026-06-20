//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ============================================================================
// SECTION 6: Transport — HandoffTransport interface + implementations
// ============================================================================

type HandoffTransport interface {
	Transfer(ctx context.Context, checksum string, snapshot []byte) error
	Receive(ctx context.Context, checksum string) ([]byte, error)
}

type TransportPayload struct {
	Checksum string `json:"checksum"`
	Data     []byte `json:"data"`
}

// InProcHandoffTransport
type InProcHandoffTransport struct {
	mu    sync.RWMutex
	store map[string][]byte
}

func NewInProcHandoffTransport() *InProcHandoffTransport {
	return &InProcHandoffTransport{store: make(map[string][]byte)}
}

func (t *InProcHandoffTransport) Transfer(ctx context.Context, checksum string, snapshot []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store[checksum] = make([]byte, len(snapshot))
	copy(t.store[checksum], snapshot)
	return nil
}

func (t *InProcHandoffTransport) Receive(ctx context.Context, checksum string) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	data, ok := t.store[checksum]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: checksum=%s", checksum)
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (t *InProcHandoffTransport) Delete(checksum string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.store, checksum)
}

func (t *InProcHandoffTransport) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.store)
}

// HTTPTransport
type HTTPTransport struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPTransport(baseURL string) *HTTPTransport {
	return &HTTPTransport{baseURL: baseURL, httpClient: &http.Client{Timeout: 30 * time.Second}}
}

func (t *HTTPTransport) Transfer(ctx context.Context, checksum string, snapshot []byte) error {
	url := fmt.Sprintf("%s/handoff/%s", t.baseURL, checksum)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(snapshot))
	if err != nil {
		return fmt.Errorf("failed to create transfer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("transfer request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("transfer failed with status %d", resp.StatusCode)
	}
	return nil
}

func (t *HTTPTransport) Receive(ctx context.Context, checksum string) ([]byte, error) {
	url := fmt.Sprintf("%s/handoff/%s", t.baseURL, checksum)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create receive request: %w", err)
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("receive request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("snapshot not found: checksum=%s", checksum)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receive failed with status %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return buf.Bytes(), nil
}

// Transport adapters
type TransportTransferAdapter struct{ transport HandoffTransport }

func NewTransportTransferAdapter(transport HandoffTransport) *TransportTransferAdapter {
	return &TransportTransferAdapter{transport: transport}
}

func (a *TransportTransferAdapter) Execute(ctx context.Context, input TransportPayload) (TransportPayload, error) {
	err := a.transport.Transfer(ctx, input.Checksum, input.Data)
	if err != nil {
		return input, fmt.Errorf("transport transfer failed: %w", err)
	}
	return input, nil
}

type TransportReceiveAdapter struct{ transport HandoffTransport }

func NewTransportReceiveAdapter(transport HandoffTransport) *TransportReceiveAdapter {
	return &TransportReceiveAdapter{transport: transport}
}

func (a *TransportReceiveAdapter) Execute(ctx context.Context, input TransportPayload) (TransportPayload, error) {
	data, err := a.transport.Receive(ctx, input.Checksum)
	if err != nil {
		return input, fmt.Errorf("transport receive failed: %w", err)
	}
	input.Data = data
	return input, nil
}
