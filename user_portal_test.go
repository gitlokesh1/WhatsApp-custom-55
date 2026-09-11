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

func TestNormalizeMobileNumber(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantValid bool
	}{
		{name: "canonical", input: "+919876543210", want: "+919876543210", wantValid: true},
		{name: "common separators", input: "+91 (9876) 543-210", want: "+919876543210", wantValid: true},
		{name: "missing country prefix", input: "9876543210", wantValid: false},
		{name: "leading zero country code", input: "+0123456789", wantValid: false},
		{name: "too short", input: "+1234567", wantValid: false},
		{name: "too long", input: "+1234567890123456", wantValid: false},
		{name: "letters", input: "+91987ABC3210", wantValid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := normalizeMobileNumber(test.input)
			if got != test.want || valid != test.wantValid {
				t.Fatalf("normalizeMobileNumber(%q) = %q, %v; want %q, %v", test.input, got, valid, test.want, test.wantValid)
			}
		})
	}
}

func TestUserRegisterRejectsInvalidMobileNumber(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"name":"Test User","password":"secret123","country_code":"IN","mobile_number":"9876543210","accepted_terms":true}`))
	recorder := httptest.NewRecorder()

	userRegisterHandler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "valid international mobile number") {
		t.Fatalf("expected mobile validation error, got %s", recorder.Body.String())
	}
}

func TestInjectBrandingScriptIsIdempotent(t *testing.T) {
	page := "<html><body><main>Page</main></body></html>"
	injected := injectBrandingScript(page)
	if strings.Count(injected, "/branding.js") != 1 {
		t.Fatalf("expected one branding script, got %q", injected)
	}
	if strings.Count(injected, "/site.webmanifest") != 1 || strings.Count(injected, "/favicon.ico") != 1 {
		t.Fatalf("expected one set of app icon links, got %q", injected)
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
