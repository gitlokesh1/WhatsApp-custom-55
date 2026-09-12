package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminSettingsRejectsInvalidRecipientCooldownBeforePersisting(t *testing.T) {
	originalDB := userDB
	userDB = nil
	t.Cleanup(func() { userDB = originalDB })

	for _, value := range []string{"not-a-number", "-1", "525601", "   "} {
		t.Run(value, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/admin/settings/data", strings.NewReader(`{"delete_chat_after_send":false,"recipient_cooldown_minutes":"`+value+`"}`))
			response := httptest.NewRecorder()

			adminSettingsAPIHandler(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected invalid cooldown %q to return 400, got %d", value, response.Code)
			}
		})
	}
}
