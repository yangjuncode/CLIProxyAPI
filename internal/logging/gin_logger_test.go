package logging

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func TestGinLogrusRecoveryRepanicsErrAbortHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusRecovery())
	engine.GET("/abort", func(c *gin.Context) {
		panic(http.ErrAbortHandler)
	})

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	recorder := httptest.NewRecorder()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic, got nil")
		}
		err, ok := recovered.(error)
		if !ok {
			t.Fatalf("expected error panic, got %T", recovered)
		}
		if !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("expected ErrAbortHandler, got %v", err)
		}
		if err != http.ErrAbortHandler {
			t.Fatalf("expected exact ErrAbortHandler sentinel, got %v", err)
		}
	}()

	engine.ServeHTTP(recorder, req)
}

func TestGinLogrusRecoveryHandlesRegularPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusRecovery())
	engine.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

func TestGinLogrusLoggerCapturesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup logrus to capture output
	var buf bytes.Buffer
	originalOutput := log.StandardLogger().Out
	log.SetOutput(&buf)
	log.SetLevel(log.DebugLevel)
	defer func() {
		log.SetOutput(originalOutput)
		log.SetLevel(log.InfoLevel)
	}()

	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.POST("/v1/responses", func(c *gin.Context) {
		c.String(http.StatusOK, "hello world")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	output := buf.String()
	if !strings.Contains(output, "Body: hello world") {
		t.Errorf("expected log to contain body, got %s", output)
	}
}

func TestGinLogrusLoggerDoesNotCaptureBodyInInfoLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup logrus to capture output
	var buf bytes.Buffer
	originalOutput := log.StandardLogger().Out
	log.SetOutput(&buf)
	log.SetLevel(log.InfoLevel)
	defer func() {
		log.SetOutput(originalOutput)
		log.SetLevel(log.InfoLevel)
	}()

	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.POST("/v1/responses", func(c *gin.Context) {
		c.String(http.StatusOK, "hello world")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	output := buf.String()
	if strings.Contains(output, "Body: hello world") {
		t.Errorf("expected log NOT to contain body in InfoLevel, got %s", output)
	}
}

func TestGinLogrusLoggerCapturesBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup logrus to capture output
	var buf bytes.Buffer
	originalOutput := log.StandardLogger().Out
	log.SetOutput(&buf)
	log.SetLevel(log.DebugLevel)
	defer func() {
		log.SetOutput(originalOutput)
		log.SetLevel(log.InfoLevel)
	}()

	engine := gin.New()
	engine.Use(GinLogrusLogger())

	longBody := strings.Repeat("a", maxResponseBodyLogSize+100)
	engine.POST("/v1/responses", func(c *gin.Context) {
		c.String(http.StatusOK, longBody)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	output := buf.String()
	expectedBody := strings.Repeat("a", maxResponseBodyLogSize)
	if !strings.Contains(output, "Body: "+expectedBody) {
		t.Errorf("expected log to contain exactly %d bytes of body", maxResponseBodyLogSize)
	}
	if strings.Contains(output, "Body: "+longBody) {
		t.Errorf("expected log NOT to contain full %d bytes of body", len(longBody))
	}
}
