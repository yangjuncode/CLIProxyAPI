package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestFilterExecutionModels_SkipsModelWhenRequestSizeExceedsLearnedMaxInput(t *testing.T) {
	t.Parallel()

	auth := &Auth{
		ModelStates: map[string]*ModelState{
			"qwen3.5-plus": {
				MaxInput:          90,
				MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
			},
		},
	}

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{
			cliproxyexecutor.RequestSizeMetadataKey: 100,
		},
	}

	got := filterExecutionModels(auth, "alias-model", []string{"qwen3.5-plus", "glm-5"}, true, opts)
	if len(got) != 1 || got[0] != "glm-5" {
		t.Fatalf("filterExecutionModels() = %v, want [glm-5]", got)
	}
}

func TestFilterExecutionModels_ExpiredMaxInputDoesNotFilter(t *testing.T) {
	t.Parallel()

	auth := &Auth{
		ModelStates: map[string]*ModelState{
			"qwen3.5-plus": {
				MaxInput:          90,
				MaxInputExpiresAt: time.Now().Add(-1 * time.Minute),
			},
		},
	}

	opts := cliproxyexecutor.Options{
		Metadata: map[string]any{
			cliproxyexecutor.RequestSizeMetadataKey: 100,
		},
	}

	got := filterExecutionModels(auth, "alias-model", []string{"qwen3.5-plus"}, true, opts)
	if len(got) != 1 || got[0] != "qwen3.5-plus" {
		t.Fatalf("filterExecutionModels() = %v, want [qwen3.5-plus]", got)
	}
}

func TestManagerExecute_SkipsModelWhenRequestSizeExceedsLearnedMaxInput(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	auth.ModelStates = map[string]*ModelState{
		"qwen3.5-plus": {
			MaxInput:          90,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
	}
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	resp, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if string(resp.Payload) != "glm-5" {
		t.Fatalf("payload = %q, want %q", string(resp.Payload), "glm-5")
	}
	if got := executor.ExecuteModels(); len(got) != 1 || got[0] != "glm-5" {
		t.Fatalf("execute calls = %v, want [glm-5]", got)
	}
}

func TestManagerExecuteCount_SkipsModelWhenRequestSizeExceedsLearnedMaxInput(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	auth.ModelStates = map[string]*ModelState{
		"qwen3.5-plus": {
			MaxInput:          90,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
	}
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	resp, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	})
	if err != nil {
		t.Fatalf("execute count error: %v", err)
	}
	if string(resp.Payload) != "glm-5" {
		t.Fatalf("payload = %q, want %q", string(resp.Payload), "glm-5")
	}
	if got := executor.CountModels(); len(got) != 1 || got[0] != "glm-5" {
		t.Fatalf("count calls = %v, want [glm-5]", got)
	}
}

func TestManagerExecuteStream_SkipsModelWhenRequestSizeExceedsLearnedMaxInput(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	auth.ModelStates = map[string]*ModelState{
		"qwen3.5-plus": {
			MaxInput:          90,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
	}
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	})
	if err != nil {
		t.Fatalf("execute stream error: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, streamResult); got != "glm-5" {
		t.Fatalf("stream payload = %q, want %q", got, "glm-5")
	}
	if got := executor.StreamModels(); len(got) != 1 || got[0] != "glm-5" {
		t.Fatalf("stream calls = %v, want [glm-5]", got)
	}
}

func TestManagerExecute_ReturnsRequestTooLargeWhenLearnedMaxInputFiltersAllModels(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	auth.ModelStates = map[string]*ModelState{
		"qwen3.5-plus": {
			MaxInput:          90,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
		"glm-5": {
			MaxInput:          95,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
	}
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	_, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	})
	if err == nil {
		t.Fatal("expected execute error")
	}
	authErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("execute error type = %T, want *Error", err)
	}
	if authErr.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("execute error status = %d, want %d", authErr.HTTPStatus, http.StatusRequestEntityTooLarge)
	}
	if authErr.Code != "request_too_large" {
		t.Fatalf("execute error code = %q, want %q", authErr.Code, "request_too_large")
	}
	if got := executor.ExecuteModels(); len(got) != 0 {
		t.Fatalf("execute calls = %v, want none", got)
	}
}

func TestManagerExecuteCount_ReturnsRequestTooLargeWhenLearnedMaxInputFiltersAllModels(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	auth.ModelStates = map[string]*ModelState{
		"qwen3.5-plus": {
			MaxInput:          90,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
		"glm-5": {
			MaxInput:          95,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
	}
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	_, err := m.ExecuteCount(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	})
	if err == nil {
		t.Fatal("expected execute count error")
	}
	authErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("execute count error type = %T, want *Error", err)
	}
	if authErr.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("execute count error status = %d, want %d", authErr.HTTPStatus, http.StatusRequestEntityTooLarge)
	}
	if authErr.Code != "request_too_large" {
		t.Fatalf("execute count error code = %q, want %q", authErr.Code, "request_too_large")
	}
	if got := executor.CountModels(); len(got) != 0 {
		t.Fatalf("count calls = %v, want none", got)
	}
}

func TestManagerExecuteStream_ReturnsRequestTooLargeWhenLearnedMaxInputFiltersAllModels(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	auth.ModelStates = map[string]*ModelState{
		"qwen3.5-plus": {
			MaxInput:          90,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
		"glm-5": {
			MaxInput:          95,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
	}
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	})
	if err == nil {
		t.Fatal("expected execute stream error")
	}
	if streamResult != nil {
		t.Fatalf("stream result = %#v, want nil", streamResult)
	}
	authErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("execute stream error type = %T, want *Error", err)
	}
	if authErr.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("execute stream error status = %d, want %d", authErr.HTTPStatus, http.StatusRequestEntityTooLarge)
	}
	if authErr.Code != "request_too_large" {
		t.Fatalf("execute stream error code = %q, want %q", authErr.Code, "request_too_large")
	}
	if got := executor.StreamModels(); len(got) != 0 {
		t.Fatalf("stream calls = %v, want none", got)
	}
}

func TestManagerExecute_DoesNotReturnRequestTooLargeWhenOnlyBlockedModelsRemain(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{id: "pool"}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	auth.ModelStates = map[string]*ModelState{
		"qwen3.5-plus": {
			Unavailable:       true,
			NextRetryAfter:    time.Now().Add(5 * time.Minute),
			MaxInput:          90,
			MaxInputExpiresAt: time.Now().Add(8 * time.Hour),
		},
		"glm-5": {
			Unavailable:    true,
			NextRetryAfter: time.Now().Add(5 * time.Minute),
		},
	}
	if _, err := m.Update(context.Background(), auth); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	_, err := m.Execute(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 100},
	})
	if err == nil {
		t.Fatal("expected execute error")
	}
	authErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("execute error type = %T, want *Error", err)
	}
	if authErr.HTTPStatus == http.StatusRequestEntityTooLarge {
		t.Fatalf("execute error status = %d, want non-413 blocked error", authErr.HTTPStatus)
	}
	if authErr.Code != "auth_not_found" {
		t.Fatalf("execute error code = %q, want %q", authErr.Code, "auth_not_found")
	}
}

func TestMarkResult_413LearnsMaxInputAndExpiry(t *testing.T) {
	m := NewManager(nil, nil, nil)
	if _, err := m.Register(context.Background(), &Auth{ID: "auth-1", Provider: "codex", Status: StatusActive}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       "gpt-5-codex",
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"},
		RequestSize: 120,
	})

	auth, ok := m.GetByID("auth-1")
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates["gpt-5-codex"]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if state.MaxInput != 110 {
		t.Fatalf("MaxInput = %d, want 110", state.MaxInput)
	}
	if state.MaxInputExpiresAt.IsZero() {
		t.Fatalf("MaxInputExpiresAt = zero, want set")
	}
}

func TestMarkResult_413DoesNotSetCooldownForSmallerRequests(t *testing.T) {
	m := NewManager(nil, nil, nil)
	if _, err := m.Register(context.Background(), &Auth{ID: "auth-1", Provider: "codex", Status: StatusActive}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	model := "gpt-5-codex"
	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"},
		RequestSize: 120,
	})

	auth, ok := m.GetByID("auth-1")
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if !state.NextRetryAfter.IsZero() {
		t.Fatalf("state.NextRetryAfter = %v, want zero", state.NextRetryAfter)
	}
	if state.Unavailable {
		t.Fatalf("state.Unavailable = true, want false")
	}
	if auth.Unavailable {
		t.Fatalf("auth.Unavailable = true, want false")
	}
	if !auth.NextRetryAfter.IsZero() {
		t.Fatalf("auth.NextRetryAfter = %v, want zero", auth.NextRetryAfter)
	}
}

func TestMarkResult_413ResumesSuspendedModelInRegistry(t *testing.T) {
	m := NewManager(nil, nil, nil)
	authID := "auth-413-resume"
	model := "gpt-5-codex"
	if _, err := m.Register(context.Background(), &Auth{ID: authID, Provider: "codex", Status: StatusActive}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient(authID, "codex", []*registry.ModelInfo{{ID: model}})
	t.Cleanup(func() {
		reg.UnregisterClient(authID)
	})

	if got := reg.GetModelCount(model); got != 1 {
		t.Fatalf("registry model count before suspension = %d, want 1", got)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:   authID,
		Provider: "codex",
		Model:    model,
		Success:  false,
		Error:    &Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota"},
	})

	authAfter429, ok := m.GetByID(authID)
	if !ok || authAfter429 == nil {
		t.Fatalf("expected auth after 429")
	}
	if blocked, reason, _ := isAuthBlockedForModel(authAfter429, model, time.Now()); !blocked || reason != blockReasonCooldown {
		t.Fatalf("blocked after 429 = %v, reason = %v, want blocked cooldown", blocked, reason)
	}
	if got := reg.GetModelCount(model); got != 0 {
		t.Fatalf("registry model count after 429 = %d, want 0", got)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:      authID,
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"},
		RequestSize: 120,
	})

	auth, ok := m.GetByID(authID)
	if !ok || auth == nil {
		t.Fatalf("expected auth after 413")
	}
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if blocked, reason, next := isAuthBlockedForModel(auth, model, time.Now()); blocked || reason != blockReasonNone || !next.IsZero() {
		t.Fatalf("blocked after 413 = %v, reason = %v, next = %v, want unblocked", blocked, reason, next)
	}
	if state.Unavailable {
		t.Fatalf("state.Unavailable = true, want false")
	}
	if !state.NextRetryAfter.IsZero() {
		t.Fatalf("state.NextRetryAfter = %v, want zero", state.NextRetryAfter)
	}
	if state.Quota.Exceeded {
		t.Fatalf("state.Quota.Exceeded = true, want false")
	}
	if auth.Unavailable {
		t.Fatalf("auth.Unavailable = true, want false")
	}
	if !auth.NextRetryAfter.IsZero() {
		t.Fatalf("auth.NextRetryAfter = %v, want zero", auth.NextRetryAfter)
	}
	if auth.Quota.Exceeded {
		t.Fatalf("auth.Quota.Exceeded = true, want false")
	}
	if got := reg.GetModelCount(model); got != 1 {
		t.Fatalf("registry model count after 413 = %d, want 1", got)
	}
}

func TestMarkResult_413KeepsLastErrorAndStatusMessage(t *testing.T) {
	m := NewManager(nil, nil, nil)
	authID := "auth-413-error"
	model := "gpt-5-codex"
	if _, err := m.Register(context.Background(), &Auth{ID: authID, Provider: "codex", Status: StatusActive}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	errTooLarge := &Error{
		Code:       "payload_too_large",
		HTTPStatus: http.StatusRequestEntityTooLarge,
		Message:    "request exceeds provider input limit",
		Retryable:  false,
	}
	m.MarkResult(context.Background(), Result{
		AuthID:      authID,
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       errTooLarge,
		RequestSize: 120,
	})

	auth, ok := m.GetByID(authID)
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if state.LastError == nil {
		t.Fatalf("state.LastError = nil, want recorded error")
	}
	if state.LastError.HTTPStatus != http.StatusRequestEntityTooLarge || state.LastError.Message != errTooLarge.Message || state.LastError.Code != errTooLarge.Code {
		t.Fatalf("state.LastError = %#v, want preserved 413 error", state.LastError)
	}
	if state.StatusMessage != errTooLarge.Message {
		t.Fatalf("state.StatusMessage = %q, want %q", state.StatusMessage, errTooLarge.Message)
	}
	if auth.LastError == nil {
		t.Fatalf("auth.LastError = nil, want recorded error")
	}
	if auth.LastError.HTTPStatus != http.StatusRequestEntityTooLarge || auth.LastError.Message != errTooLarge.Message || auth.LastError.Code != errTooLarge.Code {
		t.Fatalf("auth.LastError = %#v, want preserved 413 error", auth.LastError)
	}
	if auth.StatusMessage != errTooLarge.Message {
		t.Fatalf("auth.StatusMessage = %q, want %q", auth.StatusMessage, errTooLarge.Message)
	}
}

func TestMarkResult_413TightensMaxInputOnSmallerRequest(t *testing.T) {
	m := NewManager(nil, nil, nil)
	if _, err := m.Register(context.Background(), &Auth{ID: "auth-1", Provider: "codex", Status: StatusActive}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	model := "gpt-5-codex"
	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"},
		RequestSize: 120,
	})
	firstAuth, _ := m.GetByID("auth-1")
	firstExpiry := firstAuth.ModelStates[model].MaxInputExpiresAt

	time.Sleep(10 * time.Millisecond)

	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "still too large"},
		RequestSize: 100,
	})

	auth, ok := m.GetByID("auth-1")
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if state.MaxInput != 90 {
		t.Fatalf("MaxInput = %d, want 90", state.MaxInput)
	}
	if !state.MaxInputExpiresAt.After(firstExpiry) {
		t.Fatalf("MaxInputExpiresAt = %v, want after %v", state.MaxInputExpiresAt, firstExpiry)
	}
}

func TestMarkResult_413DoesNotLoosenMaxInputOnLargerRequest(t *testing.T) {
	m := NewManager(nil, nil, nil)
	if _, err := m.Register(context.Background(), &Auth{ID: "auth-1", Provider: "codex", Status: StatusActive}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	model := "gpt-5-codex"
	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"},
		RequestSize: 100,
	})
	firstAuth, _ := m.GetByID("auth-1")
	firstExpiry := firstAuth.ModelStates[model].MaxInputExpiresAt

	time.Sleep(10 * time.Millisecond)

	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large again"},
		RequestSize: 120,
	})

	auth, ok := m.GetByID("auth-1")
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if state.MaxInput != 90 {
		t.Fatalf("MaxInput = %d, want 90", state.MaxInput)
	}
	if !state.MaxInputExpiresAt.After(firstExpiry) {
		t.Fatalf("MaxInputExpiresAt = %v, want after %v", state.MaxInputExpiresAt, firstExpiry)
	}
}

func TestMarkResult_SuccessDoesNotClearMaxInput(t *testing.T) {
	m := NewManager(nil, nil, nil)
	if _, err := m.Register(context.Background(), &Auth{ID: "auth-1", Provider: "codex", Status: StatusActive}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	model := "gpt-5-codex"
	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       model,
		Success:     false,
		Error:       &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"},
		RequestSize: 120,
	})
	authBeforeSuccess, _ := m.GetByID("auth-1")
	firstState := authBeforeSuccess.ModelStates[model]

	m.MarkResult(context.Background(), Result{
		AuthID:   "auth-1",
		Provider: "codex",
		Model:    model,
		Success:  true,
	})

	auth, ok := m.GetByID("auth-1")
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if state.MaxInput != 110 {
		t.Fatalf("MaxInput = %d, want 110", state.MaxInput)
	}
	if !state.MaxInputExpiresAt.Equal(firstState.MaxInputExpiresAt) {
		t.Fatalf("MaxInputExpiresAt = %v, want %v", state.MaxInputExpiresAt, firstState.MaxInputExpiresAt)
	}
}

func TestManagerExecuteStream_413LearnsMaxInputFromRequestSizeMetadata(t *testing.T) {
	alias := "claude-opus-4.66"
	executor := &openAICompatPoolExecutor{
		id: "pool",
		streamFirstErrors: map[string]error{
			"qwen3.5-plus": &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"},
		},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: "qwen3.5-plus", Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	})
	if err != nil {
		t.Fatalf("execute stream error: %v", err)
	}
	if got := readOpenAICompatStreamPayload(t, streamResult); got != "glm-5" {
		t.Fatalf("stream payload = %q, want %q", got, "glm-5")
	}
	if got := executor.StreamModels(); len(got) != 2 || got[0] != "qwen3.5-plus" || got[1] != "glm-5" {
		t.Fatalf("stream calls = %v, want [qwen3.5-plus glm-5]", got)
	}

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates["qwen3.5-plus"]
	if state == nil {
		t.Fatalf("expected model state for qwen3.5-plus")
	}
	if state.MaxInput != 110 {
		t.Fatalf("MaxInput = %d, want 110", state.MaxInput)
	}
	if state.MaxInputExpiresAt.IsZero() {
		t.Fatalf("MaxInputExpiresAt = zero, want set")
	}
}

func TestManagerExecuteStream_Later413ChunkLearnsMaxInputFromRequestSizeMetadata(t *testing.T) {
	alias := "claude-opus-4.66"
	model := "qwen3.5-plus"
	streamErr := &Error{HTTPStatus: http.StatusRequestEntityTooLarge, Message: "too large"}
	executor := &openAICompatPoolExecutor{
		id: "pool",
		streamPayloads: map[string][]cliproxyexecutor.StreamChunk{
			model: {
				{Payload: []byte("partial")},
				{Err: streamErr},
			},
		},
	}
	m := newOpenAICompatPoolTestManager(t, alias, []internalconfig.OpenAICompatibilityModel{
		{Name: model, Alias: alias},
		{Name: "glm-5", Alias: alias},
	}, executor)

	streamResult, err := m.ExecuteStream(context.Background(), []string{"pool"}, cliproxyexecutor.Request{Model: alias}, cliproxyexecutor.Options{
		Metadata: map[string]any{cliproxyexecutor.RequestSizeMetadataKey: 120},
	})
	if err != nil {
		t.Fatalf("execute stream error: %v", err)
	}

	var payload []byte
	var gotErr error
	for chunk := range streamResult.Chunks {
		if chunk.Err != nil {
			if gotErr != nil {
				t.Fatalf("received multiple stream errors: first=%v second=%v", gotErr, chunk.Err)
			}
			gotErr = chunk.Err
			continue
		}
		payload = append(payload, chunk.Payload...)
	}
	if string(payload) != "partial" {
		t.Fatalf("stream payload = %q, want %q", string(payload), "partial")
	}
	if gotErr == nil {
		t.Fatal("expected later stream error")
	}
	if gotErr != streamErr {
		t.Fatalf("stream error = %v, want %v", gotErr, streamErr)
	}
	if got := executor.StreamModels(); len(got) != 1 || got[0] != model {
		t.Fatalf("stream calls = %v, want [%s]", got, model)
	}

	auth, ok := m.GetByID("pool-auth-" + t.Name())
	if !ok || auth == nil {
		t.Fatalf("expected auth to be present")
	}
	state := auth.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state for %s", model)
	}
	if state.MaxInput != 110 {
		t.Fatalf("MaxInput = %d, want 110", state.MaxInput)
	}
	if state.MaxInputExpiresAt.IsZero() {
		t.Fatalf("MaxInputExpiresAt = zero, want set")
	}
	if !state.NextRetryAfter.IsZero() {
		t.Fatalf("state.NextRetryAfter = %v, want zero", state.NextRetryAfter)
	}
	if state.Unavailable {
		t.Fatalf("state.Unavailable = true, want false")
	}
	if auth.Unavailable {
		t.Fatalf("auth.Unavailable = true, want false")
	}
	if !auth.NextRetryAfter.IsZero() {
		t.Fatalf("auth.NextRetryAfter = %v, want zero", auth.NextRetryAfter)
	}
}
