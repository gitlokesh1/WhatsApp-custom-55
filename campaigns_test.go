package main

import (
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCampaignCSV(t *testing.T) {
	csv := "phone,message,consent,title\n+919876543210,Hello,yes,Welcome\n+91 98765 43210,Hello,yes,Duplicate\n+919111111111,No consent,no,Skip me\n+919222222222,Second,true,\n"
	rows, reasons, err := parseCampaignCSV(strings.NewReader(csv), "Launch")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Title != "Welcome" || rows[1].Title != "Launch #2" {
		t.Fatalf("unexpected titles: %#v", rows)
	}
	if reasons["duplicate row"] != 1 || reasons["consent not granted"] != 1 {
		t.Fatalf("unexpected skip reasons: %#v", reasons)
	}
}

func TestValidCampaignPrice(t *testing.T) {
	for _, invalid := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if validCampaignPrice(invalid) {
			t.Fatalf("expected %v to be invalid", invalid)
		}
	}
	if !validCampaignPrice(0.0001) {
		t.Fatal("expected a finite positive price to be valid")
	}
}

func TestLikePatternEscapesMetacharacters(t *testing.T) {
	if got, want := likePattern(`a%b_c\d`), `%a\%b\_c\\d%`; got != want {
		t.Fatalf("likePattern() = %q, want %q", got, want)
	}
}

func TestParseCampaignCSVRequiresConsent(t *testing.T) {
	_, _, err := parseCampaignCSV(strings.NewReader("phone,message\n919876543210,Hello\n"), "Launch")
	if err == nil || !strings.Contains(err.Error(), "consent") {
		t.Fatalf("expected consent column error, got %v", err)
	}
}

func TestValidAdvertiserLoginID(t *testing.T) {
	for _, valid := range []string{"client-01", "agency.name", "Brand_2"} {
		if !validAdvertiserLoginID(valid) {
			t.Fatalf("expected %q to be valid", valid)
		}
	}
	for _, invalid := range []string{"ab", "bad login", "<script>"} {
		if validAdvertiserLoginID(invalid) {
			t.Fatalf("expected %q to be invalid", invalid)
		}
	}
}

func TestCampaignImportRequestSizeLimit(t *testing.T) {
	handler := requestSizeLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	payload := strings.Repeat("x", 3<<20)

	for _, path := range []string{"/admin/campaigns/import", "/api/client/campaigns/import"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("expected campaign import %s to accept 3 MiB, got %d", path, response.Code)
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(payload))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected ordinary request to keep 2 MiB limit, got %d", response.Code)
	}
}

func TestCampaignUploadErrorMessageDoesNotExposeInternalErrors(t *testing.T) {
	message := campaignUploadErrorMessage(errors.New("database connection failed for secret host"))
	if message != "Campaign upload is invalid" {
		t.Fatalf("unexpected public error message: %q", message)
	}
	if message := campaignUploadErrorMessage(errors.New("campaign upload exceeds the 64 MiB limit")); message != "campaign upload exceeds the 64 MiB limit" {
		t.Fatalf("expected a clear upload-size error, got %q", message)
	}
}

func TestAdvertiserLoginRateLimitUsesSourceAndLogin(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/client/login", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	keys := advertiserLoginRateKeys(request, " Client-01 ")
	clearAdvertiserLoginFailures(keys)
	defer clearAdvertiserLoginFailures(keys)

	for range advertiserLoginAttemptLimit {
		recordAdvertiserLoginFailure(keys)
	}
	if advertiserLoginRetryAfter(keys) <= 0 {
		t.Fatal("expected repeated failures to be rate limited")
	}

	otherSource := httptest.NewRequest(http.MethodPost, "/api/client/login", nil)
	otherSource.RemoteAddr = "192.0.2.11:1234"
	if advertiserLoginRetryAfter(advertiserLoginRateKeys(otherSource, "client-01")) <= 0 {
		t.Fatal("expected normalized login ID to remain rate limited across sources")
	}
	if advertiserLoginRetryAfter(advertiserLoginRateKeys(request, "other-client")) <= 0 {
		t.Fatal("expected request source to remain rate limited across login IDs")
	}
}
