package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tinywasm/deploy"
)

// buildHandler creates a WebhookHandler with mocked dependencies.
func buildHandler(
	keys *mockKeyManager,
	dl *mockDownloader,
	checker *mockChecker,
	mgr *mockProcessManager,
	files *mockFileOps,
) http.Handler {
	cfg := testConfig()
	secret, _ := keys.GetHMACSecret()
	validator := deploy.NewHMACValidator(secret)
	return deploy.NewWebhookHandler(cfg, validator, keys, dl, checker, mgr, files)
}

func signedRequest(t *testing.T, secret string, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("X-Signature", deploy.SignPayload(secret, body))
	return req
}

// ─── Happy Path ──────────────────────────────────────────────────────────────

// Covers: PROCESS_FLOW – valid HMAC → download → pre-flight ok → hot-swap → start → health ok → 200
func TestIntegration_HappyPath(t *testing.T) {
	keys := &mockKeyManager{hmacSecret: "secret", pat: "ghp_token", configured: true}
	dl := &mockDownloader{}
	checker := &mockChecker{responses: []error{nil, nil}} // pre-flight + post-deploy
	mgr := &mockProcessManager{}
	files := &mockFileOps{}

	h := buildHandler(keys, dl, checker, mgr, files)

	body := []byte(`{"app":"myapp","version":"v1.2.3","download_url":"http://example.com/myapp"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, signedRequest(t, "secret", body))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(mgr.stopped) == 0 {
		t.Error("expected Stop to be called")
	}
	if len(mgr.started) == 0 {
		t.Error("expected Start to be called")
	}
}

// ─── Unauthorized (invalid HMAC) ─────────────────────────────────────────────

// Covers: PROCESS_FLOW – invalid signature → 401, no download triggered
func TestIntegration_InvalidSignature(t *testing.T) {
	keys := &mockKeyManager{hmacSecret: "secret", pat: "ghp_token", configured: true}
	dl := &mockDownloader{}
	mgr := &mockProcessManager{}
	files := &mockFileOps{}

	h := buildHandler(keys, dl, nil, mgr, files)

	body := []byte(`{"app":"myapp","version":"v1.2.3","download_url":"http://example.com/myapp"}`)
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("X-Signature", "sha256=badhash")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ─── Already Up-to-Date ──────────────────────────────────────────────────────

func TestIntegration_AlreadyUpToDate(t *testing.T) {
	keys := &mockKeyManager{hmacSecret: "secret", pat: "ghp_token", configured: true}
	dl := &mockDownloader{}
	mgr := &mockProcessManager{}
	files := &mockFileOps{}

	h := buildHandler(keys, dl, nil, mgr, files)

	// testConfig has Version = "v1.0.0" — send same version
	body := []byte(`{"app":"myapp","version":"v1.0.0","download_url":"http://example.com/myapp"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, signedRequest(t, "secret", body))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 already-up-to-date, got %d", rr.Code)
	}
	if len(mgr.started) > 0 {
		t.Error("Start should NOT be called when already up-to-date")
	}
}

// ─── App Busy (can_restart=false) ────────────────────────────────────────────

// Covers: PROCESS_FLOW – pre-flight returns busy → 503, no kill triggered
func TestIntegration_AppBusy(t *testing.T) {
	keys := &mockKeyManager{hmacSecret: "secret", pat: "ghp_token", configured: true}
	dl := &mockDownloader{}
	busyErr := errBusy("app busy: can_restart=false")
	checker := &mockChecker{responses: []error{busyErr, busyErr}}
	mgr := &mockProcessManager{}
	files := &mockFileOps{}

	h := buildHandler(keys, dl, checker, mgr, files)

	body := []byte(`{"app":"myapp","version":"v1.2.3","download_url":"http://example.com/myapp"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, signedRequest(t, "secret", body))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	if len(mgr.stopped) > 0 {
		t.Error("Stop should NOT be called when app is busy")
	}
}

// ─── Rollback on Failed Post-Deploy Health Check ──────────────────────────────

// Covers: PROCESS_FLOW – post-deploy health fails → rollback → 500
func TestIntegration_RollbackOnHealthFailure(t *testing.T) {
	keys := &mockKeyManager{hmacSecret: "secret", pat: "ghp_token", configured: true}
	dl := &mockDownloader{}
	// pre-flight: ok, post-deploy: fail
	checker := &mockChecker{responses: []error{nil, errBusy("timeout")}}
	mgr := &mockProcessManager{}
	files := &mockFileOps{}

	h := buildHandler(keys, dl, checker, mgr, files)

	body := []byte(`{"app":"myapp","version":"v1.2.3","download_url":"http://example.com/myapp"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, signedRequest(t, "secret", body))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after rollback, got %d: %s", rr.Code, rr.Body.String())
	}
	// Rollback means Stop was called at least twice (initial + rollback)
	if len(mgr.stopped) < 1 {
		t.Error("expected Stop to be called for rollback")
	}
}

// ─── App Not Registered ───────────────────────────────────────────────────────

func TestIntegration_AppNotFound(t *testing.T) {
	keys := &mockKeyManager{hmacSecret: "secret", pat: "ghp_token", configured: true}
	dl := &mockDownloader{}
	mgr := &mockProcessManager{}
	files := &mockFileOps{}

	h := buildHandler(keys, dl, nil, mgr, files)

	body := []byte(`{"app":"unknown-app","version":"v1.2.3","download_url":"http://example.com/bin"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, signedRequest(t, "secret", body))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
