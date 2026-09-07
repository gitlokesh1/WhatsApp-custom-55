package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func TestRecipientLookupResponseMatchesCanonicalizedBrazilNumber(t *testing.T) {
	info := types.IsOnWhatsAppResponse{
		Query:       "+5511987654321",
		PhoneNumber: types.JID{User: "551187654321", Server: types.DefaultUserServer},
	}
	if !recipientLookupResponseMatches("5511987654321", info) {
		t.Fatal("expected the original query to match even when WhatsApp canonicalizes the returned Brazilian number")
	}
}

func TestRecipientLookupResponseRejectsDifferentQuery(t *testing.T) {
	info := types.IsOnWhatsAppResponse{
		Query:       "+919876543210",
		PhoneNumber: types.JID{User: "5511987654321", Server: types.DefaultUserServer},
	}
	if recipientLookupResponseMatches("5511987654321", info) {
		t.Fatal("expected a response for a different query to be rejected")
	}
}

func TestRecipientLookupResponseRejectsJIDOnlyResponse(t *testing.T) {
	info := types.IsOnWhatsAppResponse{
		Query: "+5511987654321",
		JID:   types.JID{User: "123456789", Server: types.HiddenUserServer},
	}
	if recipientLookupResponseMatches("5511987654321", info) {
		t.Fatal("expected a sparse JID-only response without a phone-number binding to be rejected")
	}
}

func TestWaitRecipientLookupRetryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if err := waitRecipientLookupRetry(ctx); err != context.Canceled {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if time.Since(started) >= recipientLookupRetryDelay {
		t.Fatal("retry wait did not return promptly after cancellation")
	}
}

func TestTemporaryRecipientLookupErrors(t *testing.T) {
	for _, err := range []error{context.DeadlineExceeded, whatsmeow.ErrIQTimedOut, whatsmeow.ErrIQServiceUnavailable, whatsmeow.ErrIQInternalServerError} {
		if !isTemporaryRecipientLookupError(err) {
			t.Errorf("expected %q to be treated as temporary", err)
		}
	}
	if isTemporaryRecipientLookupError(whatsmeow.ErrIQNotFound) {
		t.Fatal("not-found must not be retried as a temporary lookup failure")
	}
}

func TestTemporaryRecipientLookupFailurePreservesMessageAndType(t *testing.T) {
	err := newTemporaryRecipientLookupFailure("retry shortly")
	if err.Error() != "retry shortly" {
		t.Fatalf("expected the user-facing message to be preserved, got %q", err)
	}
	if !errors.Is(err, errTemporaryRecipientLookupFailure) {
		t.Fatal("expected the lookup failure sentinel to be discoverable")
	}
	plainSendError := errors.New("WhatsApp recipient lookup rate-limited (429); retry later")
	if errors.Is(plainSendError, errTemporaryRecipientLookupFailure) {
		t.Fatal("send-level errors must not be classified by matching their text")
	}
}
