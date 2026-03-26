package logging

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

var maxResponseBodyLogSize = 512

// bodyLogWriter is a wrapper for gin.ResponseWriter that captures the beginning
// of the response body for logging purposes.
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write intercepts the response data and copies up to maxResponseBodyLogSize
// bytes into a local buffer before passing it through to the original writer.
func (w *bodyLogWriter) Write(b []byte) (int, error) {
	if w.body.Len() < maxResponseBodyLogSize {
		remaining := maxResponseBodyLogSize - w.body.Len()
		if len(b) > remaining {
			w.body.Write(b[:remaining])
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

// NewBodyLogWriter creates a new bodyLogWriter that wraps the provided ResponseWriter.
func NewBodyLogWriter(w gin.ResponseWriter) *bodyLogWriter {
	return &bodyLogWriter{
		ResponseWriter: w,
		body:           bytes.NewBufferString(""),
	}
}

// GetCapturedBody returns the captured response body as a string.
func (w *bodyLogWriter) GetCapturedBody() string {
	return w.body.String()
}
