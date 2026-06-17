package core

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// HandoffTransport — snapshot transfer interface
// ──────────────────────────────────────────────

// HandoffTransport is the interface for transferring DevSnapshots between agents.
// It abstracts the transport mechanism (in-process, HTTP, message queue, etc.).
type HandoffTransport interface {
	// Transfer sends a snapshot identified by checksum to the target.
	// The checksum is used to verify data integrity on the receiving end.
	Transfer(ctx context.Context, checksum string, snapshot []byte) error

	// Receive retrieves a snapshot by its checksum.
	// Returns the raw snapshot bytes or an error if not found.
	Receive(ctx context.Context, checksum string) ([]byte, error)
}

// ──────────────────────────────────────────────
// InProcTransport — in-memory transport
// ──────────────────────────────────────────────

// InProcHandoffTransport stores snapshots in an in-memory map.
// Thread-safe for concurrent use. Suitable for testing and single-process deployments.
type InProcHandoffTransport struct {
	mu     sync.RWMutex
	store  map[string][]byte
}

// NewInProcHandoffTransport creates a new in-process transport.
func NewInProcHandoffTransport() *InProcHandoffTransport {
	return &InProcHandoffTransport{
		store: make(map[string][]byte),
	}
}

// Transfer stores the snapshot in the in-memory map.
func (t *InProcHandoffTransport) Transfer(ctx context.Context, checksum string, snapshot []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store[checksum] = make([]byte, len(snapshot))
	copy(t.store[checksum], snapshot)
	return nil
}

// Receive retrieves the snapshot from the in-memory map.
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

// Delete removes a snapshot from the store.
func (t *InProcHandoffTransport) Delete(checksum string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.store, checksum)
}

// Count returns the number of snapshots in the store.
func (t *InProcHandoffTransport) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.store)
}

// ──────────────────────────────────────────────
// HTTPTransport — HTTP-based transport
// ──────────────────────────────────────────────

// HTTPTransport transfers snapshots via HTTP POST/GET.
// The transfer path is "/handoff/{checksum}".
type HTTPTransport struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPTransport creates a new HTTP transport targeting the given base URL.
func NewHTTPTransport(baseURL string) *HTTPTransport {
	return &HTTPTransport{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Transfer sends the snapshot to the target via HTTP POST.
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

// Receive retrieves the snapshot from the source via HTTP GET.
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

// ──────────────────────────────────────────────
// TransportAdapter — wraps transport as Adapter
// ──────────────────────────────────────────────

// TransportTransferAdapter is an Adapter that sends a snapshot via a transport.
type TransportTransferAdapter struct {
	transport HandoffTransport
}

// NewTransportTransferAdapter creates a transfer adapter.
func NewTransportTransferAdapter(transport HandoffTransport) *TransportTransferAdapter {
	return &TransportTransferAdapter{transport: transport}
}

// Execute implements Adapter[TransportPayload, TransportPayload].
func (a *TransportTransferAdapter) Execute(ctx context.Context, input TransportPayload) (TransportPayload, error) {
	err := a.transport.Transfer(ctx, input.Checksum, input.Data)
	if err != nil {
		return input, fmt.Errorf("transport transfer failed: %w", err)
	}
	return input, nil
}

// TransportReceiveAdapter is an Adapter that receives a snapshot via a transport.
type TransportReceiveAdapter struct {
	transport HandoffTransport
}

// NewTransportReceiveAdapter creates a receive adapter.
func NewTransportReceiveAdapter(transport HandoffTransport) *TransportReceiveAdapter {
	return &TransportReceiveAdapter{transport: transport}
}

// Execute implements Adapter[TransportPayload, TransportPayload].
func (a *TransportReceiveAdapter) Execute(ctx context.Context, input TransportPayload) (TransportPayload, error) {
	data, err := a.transport.Receive(ctx, input.Checksum)
	if err != nil {
		return input, fmt.Errorf("transport receive failed: %w", err)
	}
	input.Data = data
	return input, nil
}

// TransportPayload carries data through the transport layer.
type TransportPayload struct {
	Checksum string `json:"checksum"`
	Data     []byte `json:"data"`
}