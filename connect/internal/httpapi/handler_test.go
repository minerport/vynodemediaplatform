package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPermitsSameOriginWithoutConfiguredCrossOrigin(t *testing.T) {
	h := (&Handler{}).cors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "http://connect.test/api/v1/account/login", nil)
	req.Header.Set("Origin", "http://connect.test")
	response := httptest.NewRecorder()

	h.ServeHTTP(response, req)

	if response.Code != http.StatusNoContent {
		t.Fatalf("same-origin request status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://connect.test" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSRejectsUnconfiguredCrossOrigin(t *testing.T) {
	h := (&Handler{}).cors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "http://connect.test/api/v1/account/login", nil)
	req.Header.Set("Origin", "https://evil.test")
	response := httptest.NewRecorder()

	h.ServeHTTP(response, req)

	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin request status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
