package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("код ответа = %d, ожидалось %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("тело ответа = %q, ожидалось %q", body, "ok")
	}
}

func TestHealthz_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("код ответа = %d, ожидалось %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
