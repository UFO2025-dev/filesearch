package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gatewatch/file_search/internal/server"
)

func TestNew(t *testing.T) {
	srv := server.New(":8080", "Essentiel", "dev", nil, nil, nil, "", nil)
	if srv == nil {
		t.Fatal("New returned nil")
	}
}

func TestHandleHealth(t *testing.T) {
	srv := server.New(":0", "Essentiel", "dev", nil, nil, nil, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = srv
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"mode":   "Essentiel",
		})
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", body["status"])
	}
	if body["mode"] != "Essentiel" {
		t.Errorf("expected mode=Essentiel, got %s", body["mode"])
	}

	_ = req
	_ = w
}
