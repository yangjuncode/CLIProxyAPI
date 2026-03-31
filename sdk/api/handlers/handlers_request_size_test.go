package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type metadataCaptureExecutor struct {
	mu                 sync.Mutex
	executeRequestSize int
	countRequestSize   int
	streamRequestSize  int
}

func (e *metadataCaptureExecutor) Identifier() string { return "codex" }

func (e *metadataCaptureExecutor) Execute(_ context.Context, _ *coreauth.Auth, _ coreexecutor.Request, opts coreexecutor.Options) (coreexecutor.Response, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if size, ok := opts.Metadata[coreexecutor.RequestSizeMetadataKey].(int); ok {
		e.executeRequestSize = size
	}
	return coreexecutor.Response{Payload: []byte(`{"ok":true}`)}, nil
}

func (e *metadataCaptureExecutor) ExecuteStream(_ context.Context, _ *coreauth.Auth, _ coreexecutor.Request, opts coreexecutor.Options) (*coreexecutor.StreamResult, error) {
	e.mu.Lock()
	if size, ok := opts.Metadata[coreexecutor.RequestSizeMetadataKey].(int); ok {
		e.streamRequestSize = size
	}
	e.mu.Unlock()

	chunks := make(chan coreexecutor.StreamChunk, 1)
	chunks <- coreexecutor.StreamChunk{Payload: []byte("ok")}
	close(chunks)
	return &coreexecutor.StreamResult{Chunks: chunks}, nil
}

func (e *metadataCaptureExecutor) Refresh(ctx context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
	return auth, nil
}

func (e *metadataCaptureExecutor) CountTokens(_ context.Context, _ *coreauth.Auth, _ coreexecutor.Request, opts coreexecutor.Options) (coreexecutor.Response, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if size, ok := opts.Metadata[coreexecutor.RequestSizeMetadataKey].(int); ok {
		e.countRequestSize = size
	}
	return coreexecutor.Response{Payload: []byte(`{"ok":true}`)}, nil
}

func (e *metadataCaptureExecutor) HttpRequest(context.Context, *coreauth.Auth, *http.Request) (*http.Response, error) {
	return nil, errors.New("not implemented")
}

func (e *metadataCaptureExecutor) LastExecuteRequestSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executeRequestSize
}

func (e *metadataCaptureExecutor) LastCountRequestSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.countRequestSize
}

func (e *metadataCaptureExecutor) LastStreamRequestSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.streamRequestSize
}

func newRequestSizeTestHandler(t *testing.T) (*BaseAPIHandler, *metadataCaptureExecutor) {
	t.Helper()

	manager := coreauth.NewManager(nil, nil, nil)
	executor := &metadataCaptureExecutor{}
	manager.RegisterExecutor(executor)

	if _, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:       "auth-1",
		Provider: "codex",
		Status:   coreauth.StatusActive,
	}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	registry.GetGlobalRegistry().RegisterClient("auth-1", "codex", []*registry.ModelInfo{{ID: "test-model"}})
	t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient("auth-1") })

	return NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, manager), executor
}

func TestExecuteWithAuthManager_AttachesRequestSizeMetadata(t *testing.T) {
	handler, executor := newRequestSizeTestHandler(t)
	raw := []byte(`{"model":"test-model","input":"hello"}`)

	_, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", "test-model", raw, "")
	if errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager() err = %v", errMsg)
	}

	if got := executor.LastExecuteRequestSize(); got != len(raw) {
		t.Fatalf("request size = %d, want %d", got, len(raw))
	}
}

func TestExecuteCountWithAuthManager_AttachesRequestSizeMetadata(t *testing.T) {
	handler, executor := newRequestSizeTestHandler(t)
	raw := []byte(`{"model":"test-model","input":"hello"}`)

	_, _, errMsg := handler.ExecuteCountWithAuthManager(context.Background(), "openai", "test-model", raw, "")
	if errMsg != nil {
		t.Fatalf("ExecuteCountWithAuthManager() err = %v", errMsg)
	}

	if got := executor.LastCountRequestSize(); got != len(raw) {
		t.Fatalf("request size = %d, want %d", got, len(raw))
	}
}

func TestExecuteStreamWithAuthManager_AttachesRequestSizeMetadata(t *testing.T) {
	handler, executor := newRequestSizeTestHandler(t)
	raw := []byte(`{"model":"test-model","input":"hello"}`)

	dataChan, _, errChan := handler.ExecuteStreamWithAuthManager(context.Background(), "openai", "test-model", raw, "")
	if dataChan == nil || errChan == nil {
		t.Fatalf("expected non-nil channels")
	}

	var got []byte
	for chunk := range dataChan {
		got = append(got, chunk...)
	}
	for msg := range errChan {
		if msg != nil {
			t.Fatalf("ExecuteStreamWithAuthManager() err = %v", msg)
		}
	}

	if string(got) != "ok" {
		t.Fatalf("payload = %q, want %q", string(got), "ok")
	}
	if got := executor.LastStreamRequestSize(); got != len(raw) {
		t.Fatalf("request size = %d, want %d", got, len(raw))
	}
}

func TestExecuteWithAuthManager_PrefersOriginalRequestSizeOverride(t *testing.T) {
	handler, executor := newRequestSizeTestHandler(t)
	raw := []byte(`{"model":"test-model"}`)
	ctx := WithOriginalRequestSize(context.Background(), 123)

	_, _, errMsg := handler.ExecuteWithAuthManager(ctx, "openai", "test-model", raw, "")
	if errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager() err = %v", errMsg)
	}

	if got := executor.LastExecuteRequestSize(); got != 123 {
		t.Fatalf("request size = %d, want %d", got, 123)
	}
}

func TestExecuteCountWithAuthManager_PrefersOriginalRequestSizeOverride(t *testing.T) {
	handler, executor := newRequestSizeTestHandler(t)
	raw := []byte(`{"model":"test-model"}`)
	ctx := WithOriginalRequestSize(context.Background(), 123)

	_, _, errMsg := handler.ExecuteCountWithAuthManager(ctx, "openai", "test-model", raw, "")
	if errMsg != nil {
		t.Fatalf("ExecuteCountWithAuthManager() err = %v", errMsg)
	}

	if got := executor.LastCountRequestSize(); got != 123 {
		t.Fatalf("request size = %d, want %d", got, 123)
	}
}

func TestExecuteStreamWithAuthManager_PrefersOriginalRequestSizeOverride(t *testing.T) {
	handler, executor := newRequestSizeTestHandler(t)
	raw := []byte(`{"model":"test-model"}`)
	ctx := WithOriginalRequestSize(context.Background(), 123)

	dataChan, _, errChan := handler.ExecuteStreamWithAuthManager(ctx, "openai", "test-model", raw, "")
	if dataChan == nil || errChan == nil {
		t.Fatalf("expected non-nil channels")
	}

	for range dataChan {
	}
	for msg := range errChan {
		if msg != nil {
			t.Fatalf("ExecuteStreamWithAuthManager() err = %v", msg)
		}
	}

	if got := executor.LastStreamRequestSize(); got != 123 {
		t.Fatalf("request size = %d, want %d", got, 123)
	}
}
