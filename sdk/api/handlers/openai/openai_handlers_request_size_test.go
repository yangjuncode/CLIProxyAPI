package openai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type completionsRequestSizeExecutor struct {
	mu                 sync.Mutex
	executeRequestSize int
	streamRequestSize  int
}

func (e *completionsRequestSizeExecutor) Identifier() string { return "test-provider" }

func (e *completionsRequestSizeExecutor) Execute(_ context.Context, _ *coreauth.Auth, _ coreexecutor.Request, opts coreexecutor.Options) (coreexecutor.Response, error) {
	e.mu.Lock()
	if size, ok := opts.Metadata[coreexecutor.RequestSizeMetadataKey].(int); ok {
		e.executeRequestSize = size
	}
	defer e.mu.Unlock()
	return coreexecutor.Response{
		Payload: []byte(`{"id":"cmpl-1","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`),
	}, nil
}

func (e *completionsRequestSizeExecutor) ExecuteStream(_ context.Context, _ *coreauth.Auth, _ coreexecutor.Request, opts coreexecutor.Options) (*coreexecutor.StreamResult, error) {
	e.mu.Lock()
	if size, ok := opts.Metadata[coreexecutor.RequestSizeMetadataKey].(int); ok {
		e.streamRequestSize = size
	}
	e.mu.Unlock()

	chunks := make(chan coreexecutor.StreamChunk, 1)
	chunks <- coreexecutor.StreamChunk{Payload: []byte(`{"id":"cmpl-1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`)}
	close(chunks)
	return &coreexecutor.StreamResult{Chunks: chunks}, nil
}

func (e *completionsRequestSizeExecutor) Refresh(ctx context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
	return auth, nil
}

func (e *completionsRequestSizeExecutor) CountTokens(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, errors.New("not implemented")
}

func (e *completionsRequestSizeExecutor) HttpRequest(context.Context, *coreauth.Auth, *http.Request) (*http.Response, error) {
	return nil, errors.New("not implemented")
}

func (e *completionsRequestSizeExecutor) LastExecuteRequestSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executeRequestSize
}

func (e *completionsRequestSizeExecutor) LastStreamRequestSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.streamRequestSize
}

func newOpenAICompletionsRequestSizeHandler(t *testing.T) (*OpenAIAPIHandler, *completionsRequestSizeExecutor) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	executor := &completionsRequestSizeExecutor{}
	manager := coreauth.NewManager(nil, nil, nil)
	manager.RegisterExecutor(executor)

	auth := &coreauth.Auth{ID: "auth-" + t.Name(), Provider: executor.Identifier(), Status: coreauth.StatusActive}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}
	registry.GetGlobalRegistry().RegisterClient(auth.ID, auth.Provider, []*registry.ModelInfo{{ID: "test-model"}})
	t.Cleanup(func() {
		registry.GetGlobalRegistry().UnregisterClient(auth.ID)
	})

	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, manager)
	return NewOpenAIAPIHandler(base), executor
}

func TestOpenAICompletions_PreservesOriginalRequestSizeForConvertedNonStreamRequest(t *testing.T) {
	handler, executor := newOpenAICompletionsRequestSizeHandler(t)

	router := gin.New()
	router.POST("/v1/completions", handler.Completions)

	raw := `{"model":"test-model","prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if got := executor.LastExecuteRequestSize(); got != len(raw) {
		t.Fatalf("request size = %d, want %d", got, len(raw))
	}
}

func TestOpenAICompletions_PreservesOriginalRequestSizeForConvertedStreamRequest(t *testing.T) {
	handler, executor := newOpenAICompletionsRequestSizeHandler(t)

	router := gin.New()
	router.POST("/v1/completions", handler.Completions)

	raw := `{"model":"test-model","prompt":"hello","stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if got := executor.LastStreamRequestSize(); got != len(raw) {
		t.Fatalf("request size = %d, want %d", got, len(raw))
	}
}
