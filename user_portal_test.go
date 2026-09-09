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
