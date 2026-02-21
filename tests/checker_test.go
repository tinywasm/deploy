package deploy_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tinywasm/deploy"
)

func TestCheck_Healthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "ok",
			"can_restart": true,
		})
	}))
	defer ts.Close()

	checker := deploy.NewChecker()
	status, err := checker.Check(ts.URL)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", status.Status)
	}
	if !status.CanRestart {
		t.Errorf("expected can_restart true, got false")
	}
}

func TestCheck_Busy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "busy",
			"can_restart": false,
		})
	}))
	defer ts.Close()

	checker := deploy.NewChecker()
	status, err := checker.Check(ts.URL)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status.Status != "busy" {
		t.Errorf("expected status 'busy', got '%s'", status.Status)
	}
	if status.CanRestart {
		t.Errorf("expected can_restart false, got true")
	}
}

func TestCheck_Failed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	checker := deploy.NewChecker()
	_, err := checker.Check(ts.URL)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}
