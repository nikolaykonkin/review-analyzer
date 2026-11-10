package apiclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reviews/1" {
			t.Errorf("путь = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"product_name":"Крем","status":"done"}`))
	}))
	defer server.Close()

	c := New(server.URL)
	rev, err := c.GetReview(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if rev.ID != 1 || rev.ProductName != "Крем" {
		t.Errorf("ответ = %+v", rev)
	}
}

func TestClient_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := New(server.URL)
	_, err := c.GetReview(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидалась ErrNotFound, получено: %v", err)
	}
}

func TestClient_InternalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := New(server.URL)
	_, err := c.GetStats(context.Background())
	if !errors.Is(err, ErrAPI) {
		t.Fatalf("ожидалась ErrAPI, получено: %v", err)
	}
}

func TestClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{bad}`))
	}))
	defer server.Close()

	c := New(server.URL)
	_, err := c.GetLatestReview(context.Background())
	if err == nil {
		t.Fatal("ожидалась ошибка разбора JSON")
	}
}

func TestClient_GetStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			t.Errorf("путь = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pending":1,"total":2}`))
	}))
	defer server.Close()

	c := New(server.URL)
	stats, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.Pending != 1 || stats.Total != 2 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(server.URL)
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}
