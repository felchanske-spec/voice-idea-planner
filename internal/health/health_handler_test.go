package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_Liveness(t *testing.T) {
	handler := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/health/liveness", nil)
	rr := httptest.NewRecorder()

	handler.Liveness(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Liveness must return a 200 OK status.")
	assert.Equal(t, "OK", rr.Body.String(), "The response body must be 'OK'")
}

func TestHealthHandler_Readiness_Failure(t *testing.T) {
	handler := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/health/readiness", nil)
	rr := httptest.NewRecorder()

	handler.Readiness(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Readiness should return a 503 status when the backend goes down.")
}
