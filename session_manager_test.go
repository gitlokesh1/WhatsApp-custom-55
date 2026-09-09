package main

import (
	"testing"

	"go.mau.fi/whatsmeow"
)

func TestRemoveSessionCleansPendingConversationHandler(t *testing.T) {
	const key = "test-pending-cleanup"
	client := new(whatsmeow.Client)
	session := &Session{client: client}
	registerConversationPipeline(key, client)
	conversationHandlersMu.Lock()
	handlerID, registered := conversationHandlers[client]
	conversationHandlersMu.Unlock()
	if !registered {
		t.Fatal("conversation handler was not registered")
	}

	manager.mu.Lock()
	manager.pending[key] = session
	manager.mu.Unlock()
	t.Cleanup(func() {
		removeSession(key)
		unregisterConversationPipeline(client)
	})

	if removed := removeSession(key); removed != session {
		t.Fatalf("removeSession() = %p, want pending session %p", removed, session)
	}
	conversationHandlersMu.Lock()
	_, exists := conversationHandlers[client]
	conversationHandlersMu.Unlock()
	if exists {
		t.Fatal("discarded client remains registered in conversationHandlers")
	}
	if client.RemoveEventHandler(handlerID) {
		t.Fatal("discarded client still retained the conversation callback")
	}
}
