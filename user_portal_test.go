package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserRegisterRequiresTermsAcceptance(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"name":"Test User","password":"secret123","country_code":"IN","accepted_terms":false}`))
	recorder := httptest.NewRecorder()

	userRegisterHandler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Accept the Privacy Policy") {
		t.Fatalf("expected consent error, got %s", recorder.Body.String())
	}
}

func TestInjectBrandingScriptIsIdempotent(t *testing.T) {
	page := "<html><body><main>Page</main></body></html>"
	injected := injectBrandingScript(page)
	if strings.Count(injected, "/branding.js") != 1 {
		t.Fatalf("expected one branding script, got %q", injected)
	}
	if again := injectBrandingScript(injected); again != injected {
		t.Fatal("branding script injection should be idempotent")
	}
}

func TestUserPageIncludesBrandingLoader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	recorder := httptest.NewRecorder()

	userPage(recorder, req, "user-login.html")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "/branding.js") {
		t.Fatal("expected the shared branding loader in the rendered page")
	}
}

func TestPublicRegisterBannersRejectInvalidCountry(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/public/banners?placement=register&country=INVALID", nil)
	recorder := httptest.NewRecorder()

	publicBannersHandler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Invalid country code") {
		t.Fatalf("expected country validation error, got %s", recorder.Body.String())
	}
}
