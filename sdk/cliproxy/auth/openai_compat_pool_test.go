package auth

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type openAICompatPoolExecutor struct {
	id string

	mu                sync.Mutex
	executeModels     []string
	countModels       []string
	streamModels      []string
	executeErrors     map[string]error
	countErrors       map[string]error
	streamFirstErrors map[string]error
	streamPayloads    map[string][]cliproxyexecutor.StreamChunk
}

func (e *openAICompatPoolExecutor) Identifier() string { return e.id }

func (e *openAICompatPoolExecutor) Execute(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	_ = ctx
	_ = auth
	_ = opts
	e.mu.Lock()
	e.executeModels = append(e.executeModels, req.Model)
	err := e.executeErrors[req.Model]
	e.mu.Unlock()
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	return cliproxyexecutor.Response{Payload: []byte(req.Model)}, nil
}

func (e *openAICompatPoolExecutor) ExecuteStream(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	_ = ctx
	_ = auth
	_ = opts
	e.mu.Lock()
	e.streamModels = append(e.streamModels, req.Model)
	err := e.streamFirstErrors[req.Model]
	payloadChunks, hasCustomChunks := e.streamPayloads[req.Model]
	chunks := append([]cliproxyexecutor.StreamChunk(nil), payloadChunks...)
	e.mu.Unlock()
	ch := make(chan cliproxyexecutor.StreamChunk, max(1, len(chunks)))
	if err != nil {
		ch <- cliproxyexecutor.StreamChunk{Err: err}
		close(ch)
		return &cliproxyexecutor.StreamResult{Headers: http.Header{"X-Model": {req.Model}}, Chunks: ch}, nil
	}
	if !hasCustomChunks {
		ch <- cliproxyexecutor.StreamChunk{Payload: []byte(req.Model)}
	} else {
		for _, chunk := range chunks {
			ch <- chunk
		}
	}
	close(ch)
	return &cliproxyexecutor.StreamResult{Headers: http.Header{"X-Model": {req.Model}}, Chunks: ch}, nil
}

func (e *openAICompatPoolExecutor) Refresh(_ context.Context, auth *Auth) (*Auth, error) {
	return auth, nil
}

func (e *openAICompatPoolExecutor) CountTokens(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	_ = ctx
	_ = auth
	_ = opts
	e.mu.Lock()
	e.countModels = append(e.countModels, req.Model)
	err := e.countErrors[req.Model]
	e.mu.Unlock()
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	return cliproxyexecutor.Response{Payload: []byte(req.Model)}, nil
}

func (e *openAICompatPoolExecutor) HttpRequest(ctx context.Context, auth *Auth, req *http.Request) (*http.Response, error) {
	_ = ctx
	_ = auth
	_ = req
	return nil, &Error{HTTPStatus: http.StatusNotImplemented, Message: "HttpRequest not implemented"}
}

func (e *openAICompatPoolExecutor) ExecuteModels() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.executeModels))
	copy(out, e.executeModels)
	return out
}

func (e *openAICompatPoolExecutor) CountModels() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.countModels))
	copy(out, e.countModels)
	return out
}

func (e *openAICompatPoolExecutor) StreamModels() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.streamModels))
	copy(out, e.streamModels)
	return out
}

type authScopedOpenAICompatPoolExecutor struct {
	id string

	mu           sync.Mutex
	executeCalls []string
}

func (e *authScopedOpenAICompatPoolExecutor) Identifier() string { return e.id }

func (e *authScopedOpenAICompatPoolExecutor) Execute(_ context.Context, auth *Auth, req cliproxyexecutor.Request, _ cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	call := auth.ID + "|" + req.Model
	e.mu.Lock()
	e.executeCalls = append(e.executeCalls, call)
	e.mu.Unlock()
	return cliproxyexecutor.Response{Payload: []byte(call)}, nil
}

func (e *authScopedOpenAICompatPoolExecutor) ExecuteStream(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, &Error{HTTPStatus: http.StatusNotImplemented, Message: "ExecuteStream not implemented"}
}

func (e *authScopedOpenAICompatPoolExecutor) Refresh(_ context.Context, auth *Auth) (*Auth, error) {
	return auth, nil
}

func (e *authScopedOpenAICompatPoolExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, &Error{HTTPStatus: http.StatusNotImplemented, Message: "CountTokens not implemented"}
}

func (e *authScopedOpenAICompatPoolExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, &Error{HTTPStatus: http.StatusNotImplemented, Message: "HttpRequest not implemented"}
}

func (e *authScopedOpenAICompatPoolExecutor) ExecuteCalls() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.executeCalls))
	copy(out, e.executeCalls)
	return out
}

func newOpenAICompatPoolTestManager(t *testing.T, alias string, models []internalconfig.OpenAICompatibilityModel, executor *openAICompatPoolExecutor) *Manager {
	t.Helper()
	cfg := &internalconfig.Config{
		OpenAICompatibility: []internalconfig.OpenAICompatibility{{
			Name:   "pool",
			Models: models,
		}},
	}
	m := NewManager(nil, nil, nil)
	m.SetConfig(cfg)
	if executor == nil {
		executor = &openAICompatPoolExecutor{id: "pool"}
	}
	m.RegisterExecutor(executor)

	auth := &Auth{
		ID:       "pool-auth-" + t.Name(),
		Provider: "pool",
		Status:   StatusActive,
		Attributes: map[string]string{
			"api_key":      "test-key",
			"compat_name":  "pool",
			"provider_key": "pool",
		},
	}
	if _, err := m.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient(auth.ID, "pool", []*registry.ModelInfo{{ID: alias}})
	t.Cleanup(func() {
		reg.UnregisterClient(auth.ID)
	})
	return m
}

func getOpenAICompatPoolTestAuth(t *testing.T, m *Manager) *Auth {
	t.Helper()
	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	return auth
}

func forceOpenAICompatAliasPoolFront(t *testing.T, m *Manager, auth *Auth, requestedModel string) {
	t.Helper()
	if m == nil {
		t.Fatal("expected manager")
	}
	if auth == nil {
		t.Fatal("expected auth")
	}
	key := openAICompatModelPoolKey(auth, requestedModel)
	m.mu.Lock()
	if m.modelPoolOffsets == nil {
		m.modelPoolOffsets = make(map[string]int)
	}
	m.modelPoolOffsets[key] = 0
	m.mu.Unlock()
}

func assertOpenAICompatLearnedMaxInputState(t *testing.T, m *Manager, model string, wantMaxInput int) *Auth {
	t.Helper()
	auth := getOpenAICompatPoolTestAuth(t, m)
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state for %s", model)
	}
	if state.MaxInput != wantMaxInput {
		t.Fatalf("state.MaxInput = %d, want %d", state.MaxInput, wantMaxInput)
	}
	if state.MaxInputExpiresAt.IsZero() {
		t.Fatalf("state.MaxInputExpiresAt = zero, want set")
	}
	now := time.Now()
	if !state.MaxInputExpiresAt.After(now) {
		t.Fatalf("state.MaxInputExpiresAt = %v, want after %v", state.MaxInputExpiresAt, now)
	}
	if state.Unavailable {
		t.Fatalf("state.Unavailable = true, want false")
	}
	if !state.NextRetryAfter.IsZero() {
		t.Fatalf("state.NextRetryAfter = %v, want zero", state.NextRetryAfter)
	}
	return auth
}

func readOpenAICompatStreamPayload(t *testing.T, streamResult *cliproxyexecutor.StreamResult) string {
	t.Helper()
	if streamResult == nil {
		t.Fatal("expected stream result")
	}
	var payload []byte
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			t.Fatalf("unexpected stream error: %v", chunk.Err)
		}
		payload = append(payload, chunk.Payload...)
	}
	return string(payload)
}

func TestManagerExecuteCount_OpenAICompatAliasPoolStopsOnInvalidRequest(t *testing.T) {
	alias := "claude-opus-4.66"
	invalidErr := &Error{HTTPStatus: http.StatusUnprocessableEntity, Message: "unprocessable entity"}
	executor := &openAICompatPoolExecutor{
		id:          "pool",
		countErrors: map[string]error{"qwen3.5-plus": invalidErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	_, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err == nil || err.Error() != invalidErr.Error() {
		t.Fatalf("execute count error = %v, want %v", err, invalidErr)
	}
	got := executor.CountModels()
	if len(got) != 1 || got[0] != "qwen3.5-plus" {
		t.Fatalf("count calls = %v, want only first invalid model", got)
	}
}
func TestResolveModelAliasPoolFromConfigModels(t *testing.T) {
	models := []modelAliasEntry{
		internalconfig.OpenAICompatibilityModel{Name: "qwen3.5-plus", Alias: "claude-opus-4.66"},
		internalconfig.OpenAICompatibilityModel{Name: "glm-5", Alias: "claude-opus-4.66"},
		internalconfig.OpenAICompatibilityModel{Name: "kimi-k2.5", Alias: "claude-opus-4.66"},
	}
	got := resolveModelAliasPoolFromConfigModels("claude-opus-4.66(8192)", models)
	want := []string{"qwen3.5-plus(8192)", "glm-5(8192)", "kimi-k2.5(8192)"}
	if len(got) != len(want) {
		t.Fatalf("pool len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pool[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecute_OpenAICompatAliasPoolRotatesWithinAuth(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	for i := 0; i < 3; i++ {
		resp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
		if err != nil {
			t.Fatalf("execute %d: %v", i, err)
		}
		if len(resp.Payload) == 0 {
			t.Fatalf("execute %d returned empty payload", i)
		}
	}

	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5", "qwen3.5-plus"}
	if len(got) != len(want) {
		t.Fatalf("execute calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("execute call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecute_OpenAICompatAliasPoolStopsOnBadRequest(t *testing.T) {
	alias := "claude-opus-4.66"
	invalidErr := &Error{HTTPStatus: http.StatusBadRequest, Message: "invalid_request_error: malformed payload"}
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": invalidErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	_, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err == nil || err.Error() != invalidErr.Error() {
		t.Fatalf("execute error = %v, want %v", err, invalidErr)
	}
	got := executor.ExecuteModels()
	if len(got) != 1 || got[0] != "qwen3.5-plus" {
		t.Fatalf("execute calls = %v, want only first invalid model", got)
	}
}

func TestManagerExecute_OpenAICompatAliasPoolFallsBackOnModelSupportBadRequest(t *testing.T) {
	alias := "claude-opus-4.66"
	modelSupportErr := &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "invalid_request_error: The requested model is not supported.",
	}
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": modelSupportErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	resp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("execute error = %v, want fallback success", err)
	}
	if string(resp.Payload) != "glm-5" {
		t.Fatalf("payload = %q, want %q", string(resp.Payload), "glm-5")
	}
	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5"}
	if len(got) != len(want) {
		t.Fatalf("execute calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("execute call %d model = %q, want %q", i, got[i], want[i])
		}
	}

	updated, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain registered")
	}
	state := updated.ModelStates["qwen3.5-plus"]
	if state == nil {
		t.Fatalf("expected suspended upstream model state")
	}
	if !state.Unavailable || state.NextRetryAfter.IsZero() {
		t.Fatalf("expected upstream model suspension, got %+v", state)
	}
}

func TestManagerExecute_OpenAICompatAliasPoolFallsBackOnModelSupportUnprocessableEntity(t *testing.T) {
	alias := "claude-opus-4.66"
	modelSupportErr := &Error{
		HTTPStatus: http.StatusUnprocessableEntity,
		Message:    "The requested model is not supported.",
	}
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": modelSupportErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	resp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("execute error = %v, want fallback success", err)
	}
	if string(resp.Payload) != "glm-5" {
		t.Fatalf("payload = %q, want %q", string(resp.Payload), "glm-5")
	}
	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5"}
	if len(got) != len(want) {
		t.Fatalf("execute calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("execute call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecute_OpenAICompatAliasPoolFallsBackWithinSameAuth(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": &Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota"}},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	resp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if string(resp.Payload) != "glm-5" {
		t.Fatalf("payload = %q, want %q", string(resp.Payload), "glm-5")
	}
	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("execute call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolRetriesOnEmptyBootstrap(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{
		id: "pool",
		streamPayloads: map[string][]cliproxyexecutor.StreamChunk{
			"qwen3.5-plus": {},
		},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("execute stream: %v", err)
	}
	var payload []byte
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			t.Fatalf("unexpected stream error: %v", chunk.Err)
		}
		payload = append(payload, chunk.Payload...)
	}
	if string(payload) != "glm-5" {
		t.Fatalf("payload = %q, want %q", string(payload), "glm-5")
	}
	got := executor.StreamModels()
	want := []string{"qwen3.5-plus", "glm-5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stream call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolFallsBackBeforeFirstByte(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{
		id:                "pool",
		streamFirstErrors: map[string]error{"qwen3.5-plus": &Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota"}},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("execute stream: %v", err)
	}
	var payload []byte
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			t.Fatalf("unexpected stream error: %v", chunk.Err)
		}
		payload = append(payload, chunk.Payload...)
	}
	if string(payload) != "glm-5" {
		t.Fatalf("payload = %q, want %q", string(payload), "glm-5")
	}
	got := executor.StreamModels()
	want := []string{"qwen3.5-plus", "glm-5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stream call %d model = %q, want %q", i, got[i], want[i])
		}
	}
	if gotHeader := streamResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
		t.Fatalf("header X-Model = %q, want %q", gotHeader, "glm-5")
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolStopsOnInvalidRequest(t *testing.T) {
	alias := "claude-opus-4.66"
	invalidErr := &Error{HTTPStatus: http.StatusUnprocessableEntity, Message: "unprocessable entity"}
	executor := &openAICompatPoolExecutor{
		id:                "pool",
		streamFirstErrors: map[string]error{"qwen3.5-plus": invalidErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	_, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err == nil || err.Error() != invalidErr.Error() {
		t.Fatalf("execute stream error = %v, want %v", err, invalidErr)
	}
	got := executor.StreamModels()
	if len(got) != 1 || got[0] != "qwen3.5-plus" {
		t.Fatalf("stream calls = %v, want only first invalid model", got)
	}
}

func TestManagerExecute_OpenAICompatAliasPoolSkipsSuspendedUpstreamOnLaterRequests(t *testing.T) {
	alias := "claude-opus-4.66"
	modelSupportErr := &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "invalid_request_error: The requested model is not supported.",
	}
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": modelSupportErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	for i := 0; i < 3; i++ {
		resp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
		if err != nil {
			t.Fatalf("execute %d: %v", i, err)
		}
		if string(resp.Payload) != "glm-5" {
			t.Fatalf("execute %d payload = %q, want %q", i, string(resp.Payload), "glm-5")
		}
	}

	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "glm-5"}
	if len(got) != len(want) {
		t.Fatalf("execute calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("execute call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecute_OpenAICompatAliasPoolSkipsLearnedOversizeModelOnLaterRequests(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}

	firstResp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	if string(firstResp.Payload) != "glm-5" {
		t.Fatalf("first payload = %q, want %q", string(firstResp.Payload), "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}
	if string(secondResp.Payload) != "glm-5" {
		t.Fatalf("second payload = %q, want %q", string(secondResp.Payload), "glm-5")
	}

	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	thirdResp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("third execute: %v", err)
	}
	if string(thirdResp.Payload) != "glm-5" {
		t.Fatalf("third payload = %q, want %q", string(thirdResp.Payload), "glm-5")
	}

	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "glm-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("execute calls = %v, want %v", got, want)
	}
}

func TestManagerExecute_OpenAICompatAliasPoolAllowsLearnedModelForSmallerLaterRequest(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	largeOpts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}
	smallOpts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	}

	firstResp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, largeOpts)
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	if string(firstResp.Payload) != "glm-5" {
		t.Fatalf("first payload = %q, want %q", string(firstResp.Payload), "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	delete(executor.executeErrors, "qwen3.5-plus")
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, smallOpts)
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}
	if string(secondResp.Payload) != "qwen3.5-plus" {
		t.Fatalf("second payload = %q, want %q", string(secondResp.Payload), "qwen3.5-plus")
	}

	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5", "qwen3.5-plus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("execute calls = %v, want %v", got, want)
	}
}

func TestManagerExecute_OpenAICompatAliasPoolAllowsModelAgainAfterLearnedLimitExpires(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:            "pool",
		executeErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}

	if _, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts); err != nil {
		t.Fatalf("first execute: %v", err)
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("second execute before expiry: %v", err)
	}
	if string(secondResp.Payload) != "glm-5" {
		t.Fatalf("second payload before expiry = %q, want %q", string(secondResp.Payload), "glm-5")
	}
	if got := executor.ExecuteModels(); !reflect.DeepEqual(got, []string{"qwen3.5-plus", "glm-5", "glm-5"}) {
		t.Fatalf("execute calls before expiry = %v, want %v", got, []string{"qwen3.5-plus", "glm-5", "glm-5"})
	}

	state := auth.ModelStates["qwen3.5-plus"]
	if state == nil {
		t.Fatalf("expected model state for qwen3.5-plus")
	}
	state.MaxInputExpiresAt = time.Now().Add(-1 * time.Minute)
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	delete(executor.executeErrors, "qwen3.5-plus")
	auth = getOpenAICompatPoolTestAuth(t, m)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	thirdResp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("third execute: %v", err)
	}
	if string(thirdResp.Payload) != "qwen3.5-plus" {
		t.Fatalf("third payload = %q, want %q", string(thirdResp.Payload), "qwen3.5-plus")
	}

	got := executor.ExecuteModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "qwen3.5-plus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("execute calls = %v, want %v", got, want)
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolSkipsSuspendedUpstreamOnLaterRequests(t *testing.T) {
	alias := "claude-opus-4.66"
	modelSupportErr := &Error{
		HTTPStatus: http.StatusUnprocessableEntity,
		Message:    "The requested model is not supported.",
	}
	executor := &openAICompatPoolExecutor{
		id:                "pool",
		streamFirstErrors: map[string]error{"qwen3.5-plus": modelSupportErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	for i := 0; i < 3; i++ {
		streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
		if err != nil {
			t.Fatalf("execute stream %d: %v", i, err)
		}
		if payload := readOpenAICompatStreamPayload(t, streamResult); payload != "glm-5" {
			t.Fatalf("execute stream %d payload = %q, want %q", i, payload, "glm-5")
		}
		if gotHeader := streamResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
			t.Fatalf("execute stream %d header X-Model = %q, want %q", i, gotHeader, "glm-5")
		}
	}

	got := executor.StreamModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "glm-5"}
	if len(got) != len(want) {
		t.Fatalf("stream calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stream call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolSkipsLearnedOversizeModelOnLaterRequests(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:                "pool",
		streamFirstErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}

	firstResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("first execute stream: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, firstResult); got != "glm-5" {
		t.Fatalf("first stream payload = %q, want %q", got, "glm-5")
	}
	if gotHeader := firstResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
		t.Fatalf("first stream header X-Model = %q, want %q", gotHeader, "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("second execute stream: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, secondResult); got != "glm-5" {
		t.Fatalf("second stream payload = %q, want %q", got, "glm-5")
	}
	if gotHeader := secondResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
		t.Fatalf("second stream header X-Model = %q, want %q", gotHeader, "glm-5")
	}

	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	thirdResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("third execute stream: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, thirdResult); got != "glm-5" {
		t.Fatalf("third stream payload = %q, want %q", got, "glm-5")
	}
	if gotHeader := thirdResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
		t.Fatalf("third stream header X-Model = %q, want %q", gotHeader, "glm-5")
	}

	got := executor.StreamModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "glm-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stream calls = %v, want %v", got, want)
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolAllowsLearnedModelForSmallerLaterRequest(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:                "pool",
		streamFirstErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	largeOpts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}
	smallOpts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	}

	firstResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, largeOpts)
	if err != nil {
		t.Fatalf("first execute stream: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, firstResult); got != "glm-5" {
		t.Fatalf("first stream payload = %q, want %q", got, "glm-5")
	}
	if gotHeader := firstResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
		t.Fatalf("first stream header X-Model = %q, want %q", gotHeader, "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	delete(executor.streamFirstErrors, "qwen3.5-plus")
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, smallOpts)
	if err != nil {
		t.Fatalf("second execute stream: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, secondResult); got != "qwen3.5-plus" {
		t.Fatalf("second stream payload = %q, want %q", got, "qwen3.5-plus")
	}
	if gotHeader := secondResult.Headers.Get("X-Model"); gotHeader != "qwen3.5-plus" {
		t.Fatalf("second stream header X-Model = %q, want %q", gotHeader, "qwen3.5-plus")
	}

	got := executor.StreamModels()
	want := []string{"qwen3.5-plus", "glm-5", "qwen3.5-plus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stream calls = %v, want %v", got, want)
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolAllowsModelAgainAfterLearnedLimitExpires(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:                "pool",
		streamFirstErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}

	firstResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("first execute stream: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, firstResult); got != "glm-5" {
		t.Fatalf("first stream payload = %q, want %q", got, "glm-5")
	}
	if gotHeader := firstResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
		t.Fatalf("first stream header X-Model = %q, want %q", gotHeader, "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("second execute stream before expiry: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, secondResult); got != "glm-5" {
		t.Fatalf("second stream payload before expiry = %q, want %q", got, "glm-5")
	}
	if gotHeader := secondResult.Headers.Get("X-Model"); gotHeader != "glm-5" {
		t.Fatalf("second stream header X-Model before expiry = %q, want %q", gotHeader, "glm-5")
	}
	if got := executor.StreamModels(); !reflect.DeepEqual(got, []string{"qwen3.5-plus", "glm-5", "glm-5"}) {
		t.Fatalf("stream calls before expiry = %v, want %v", got, []string{"qwen3.5-plus", "glm-5", "glm-5"})
	}

	state := auth.ModelStates["qwen3.5-plus"]
	if state == nil {
		t.Fatalf("expected model state for qwen3.5-plus")
	}
	state.MaxInputExpiresAt = time.Now().Add(-1 * time.Minute)
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	delete(executor.streamFirstErrors, "qwen3.5-plus")
	auth = getOpenAICompatPoolTestAuth(t, m)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	thirdResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("third execute stream: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, thirdResult); got != "qwen3.5-plus" {
		t.Fatalf("third stream payload = %q, want %q", got, "qwen3.5-plus")
	}
	if gotHeader := thirdResult.Headers.Get("X-Model"); gotHeader != "qwen3.5-plus" {
		t.Fatalf("third stream header X-Model = %q, want %q", gotHeader, "qwen3.5-plus")
	}

	got := executor.StreamModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "qwen3.5-plus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stream calls = %v, want %v", got, want)
	}
}

func TestManagerExecuteCount_OpenAICompatAliasPoolRotatesWithinAuth(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	for i := 0; i < 2; i++ {
		resp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
		if err != nil {
			t.Fatalf("execute count %d: %v", i, err)
		}
		if len(resp.Payload) == 0 {
			t.Fatalf("execute count %d returned empty payload", i)
		}
	}

	got := executor.CountModels()
	want := []string{"qwen3.5-plus", "glm-5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("count call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecuteCount_OpenAICompatAliasPoolSkipsSuspendedUpstreamOnLaterRequests(t *testing.T) {
	alias := "claude-opus-4.66"
	modelSupportErr := &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "invalid_request_error: The requested model is unsupported.",
	}
	executor := &openAICompatPoolExecutor{
		id:          "pool",
		countErrors: map[string]error{"qwen3.5-plus": modelSupportErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	for i := 0; i < 3; i++ {
		resp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
		if err != nil {
			t.Fatalf("execute count %d: %v", i, err)
		}
		if string(resp.Payload) != "glm-5" {
			t.Fatalf("execute count %d payload = %q, want %q", i, string(resp.Payload), "glm-5")
		}
	}

	got := executor.CountModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "glm-5"}
	if len(got) != len(want) {
		t.Fatalf("count calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("count call %d model = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestManagerExecuteCount_OpenAICompatAliasPoolSkipsLearnedOversizeModelOnLaterRequests(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:          "pool",
		countErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}

	firstResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("first execute count: %v", err)
	}
	if string(firstResp.Payload) != "glm-5" {
		t.Fatalf("first payload = %q, want %q", string(firstResp.Payload), "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("second execute count: %v", err)
	}
	if string(secondResp.Payload) != "glm-5" {
		t.Fatalf("second payload = %q, want %q", string(secondResp.Payload), "glm-5")
	}

	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	thirdResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("third execute count: %v", err)
	}
	if string(thirdResp.Payload) != "glm-5" {
		t.Fatalf("third payload = %q, want %q", string(thirdResp.Payload), "glm-5")
	}

	got := executor.CountModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "glm-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("count calls = %v, want %v", got, want)
	}
}

func TestManagerExecuteCount_OpenAICompatAliasPoolAllowsLearnedModelForSmallerLaterRequest(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:          "pool",
		countErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	largeOpts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}
	smallOpts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	}

	firstResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, largeOpts)
	if err != nil {
		t.Fatalf("first execute count: %v", err)
	}
	if string(firstResp.Payload) != "glm-5" {
		t.Fatalf("first payload = %q, want %q", string(firstResp.Payload), "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	delete(executor.countErrors, "qwen3.5-plus")
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, smallOpts)
	if err != nil {
		t.Fatalf("second execute count: %v", err)
	}
	if string(secondResp.Payload) != "qwen3.5-plus" {
		t.Fatalf("second payload = %q, want %q", string(secondResp.Payload), "qwen3.5-plus")
	}

	got := executor.CountModels()
	want := []string{"qwen3.5-plus", "glm-5", "qwen3.5-plus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("count calls = %v, want %v", got, want)
	}
}

func TestManagerExecuteCount_OpenAICompatAliasPoolAllowsModelAgainAfterLearnedLimitExpires(t *testing.T) {
	alias := "claude-opus-4.66"
	tooLarge := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id:          "pool",
		countErrors: map[string]error{"qwen3.5-plus": tooLarge},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	}

	firstResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("first execute count: %v", err)
	}
	if string(firstResp.Payload) != "glm-5" {
		t.Fatalf("first payload = %q, want %q", string(firstResp.Payload), "glm-5")
	}

	auth := assertOpenAICompatLearnedMaxInputState(t, m, "qwen3.5-plus", 110)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	secondResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("second execute count before expiry: %v", err)
	}
	if string(secondResp.Payload) != "glm-5" {
		t.Fatalf("second payload before expiry = %q, want %q", string(secondResp.Payload), "glm-5")
	}
	if got := executor.CountModels(); !reflect.DeepEqual(got, []string{"qwen3.5-plus", "glm-5", "glm-5"}) {
		t.Fatalf("count calls before expiry = %v, want %v", got, []string{"qwen3.5-plus", "glm-5", "glm-5"})
	}

	state := auth.ModelStates["qwen3.5-plus"]
	if state == nil {
		t.Fatalf("expected model state for qwen3.5-plus")
	}
	state.MaxInputExpiresAt = time.Now().Add(-1 * time.Minute)
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	delete(executor.countErrors, "qwen3.5-plus")
	auth = getOpenAICompatPoolTestAuth(t, m)
	forceOpenAICompatAliasPoolFront(t, m, auth, alias)

	thirdResp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, opts)
	if err != nil {
		t.Fatalf("third execute count: %v", err)
	}
	if string(thirdResp.Payload) != "qwen3.5-plus" {
		t.Fatalf("third payload = %q, want %q", string(thirdResp.Payload), "qwen3.5-plus")
	}

	got := executor.CountModels()
	want := []string{"qwen3.5-plus", "glm-5", "glm-5", "qwen3.5-plus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("count calls = %v, want %v", got, want)
	}
}

func TestManagerExecute_OpenAICompatAliasPoolBlockedAuthDoesNotConsumeRetryBudget(t *testing.T) {
	alias := "claude-opus-4.66"
	cfg := &internalconfig.Config{
		OpenAICompatibility: []internalconfig.OpenAICompatibility{{
			Name: "pool",
			Models: []internalconfig.OpenAICompatibilityModel{
				{Name: "qwen3.5-plus", Alias: alias},
				{Name: "glm-5", Alias: alias},
			},
		}},
	}
	m := NewManager(nil, nil, nil)
	m.SetConfig(cfg)
	m.SetRetryConfig(0, 0, 1)

	executor := &authScopedOpenAICompatPoolExecutor{id: "pool"}
	m.RegisterExecutor(executor)

	badAuth := &Auth{
		ID:       "aa-blocked-auth",
		Provider: "pool",
		Status:   StatusActive,
		Attributes: map[string]string{
			"api_key":      "bad-key",
			"compat_name":  "pool",
			"provider_key": "pool",
		},
	}
	goodAuth := &Auth{
		ID:       "bb-good-auth",
		Provider: "pool",
		Status:   StatusActive,
		Attributes: map[string]string{
			"api_key":      "good-key",
			"compat_name":  "pool",
			"provider_key": "pool",
		},
	}
	if _, err := m.Register(context.Background(), badAuth); err != nil {
		t.Fatalf("register bad auth: %v", err)
	}
	if _, err := m.Register(context.Background(), goodAuth); err != nil {
		t.Fatalf("register good auth: %v", err)
	}

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient(badAuth.ID, "pool", []*registry.ModelInfo{{ID: alias}})
	reg.RegisterClient(goodAuth.ID, "pool", []*registry.ModelInfo{{ID: alias}})
	t.Cleanup(func() {
		reg.UnregisterClient(badAuth.ID)
		reg.UnregisterClient(goodAuth.ID)
	})

	modelSupportErr := &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    "invalid_request_error: The requested model is not supported.",
	}
	for _, upstreamModel := range []string{"qwen3.5-plus", "glm-5"} {
		m.MarkResult(context.Background(), Result{
			AuthID:   badAuth.ID,
			Provider: "pool",
			Model:    upstreamModel,
			Success:  false,
			Error:    modelSupportErr,
		})
	}

	resp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("execute error = %v, want success via fallback auth", err)
	}
	if !strings.HasPrefix(string(resp.Payload), goodAuth.ID+"|") {
		t.Fatalf("payload = %q, want auth %q", string(resp.Payload), goodAuth.ID)
	}

	got := executor.ExecuteCalls()
	if len(got) != 1 {
		t.Fatalf("execute calls = %v, want only one real execution on fallback auth", got)
	}
	if !strings.HasPrefix(got[0], goodAuth.ID+"|") {
		t.Fatalf("execute call = %q, want fallback auth %q", got[0], goodAuth.ID)
	}
}

func TestManagerExecuteStream_OpenAICompatAliasPoolStopsOnInvalidBootstrap(t *testing.T) {
	alias := "claude-opus-4.66"
	invalidErr := &Error{HTTPStatus: http.StatusBadRequest, Message: "invalid_request_error: malformed payload"}
	executor := &openAICompatPoolExecutor{
		id:                "pool",
		streamFirstErrors: map[string]error{"qwen3.5-plus": invalidErr},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{})
	if err == nil {
		t.Fatal("expected invalid request error")
	}
	if err != invalidErr {
		t.Fatalf("error = %v, want %v", err, invalidErr)
	}
	if streamResult != nil {
		t.Fatalf("streamResult = %#v, want nil on invalid bootstrap", streamResult)
	}
	if got := executor.StreamModels(); len(got) != 1 || got[0] != "qwen3.5-plus" {
		t.Fatalf("stream calls = %v, want only first upstream model", got)
	}
}
