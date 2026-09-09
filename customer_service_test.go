package main

import "testing"

func TestCustomerServiceToken(t *testing.T) {
	token, err := newCustomerServiceToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}
	if got := customerServiceTokenHash(token); len(got) != 64 || got == token {
		t.Fatalf("unexpected token hash %q", got)
	}
	if customerServiceTokenHash(token) != customerServiceTokenHash(token) {
		t.Fatal("token hash is not deterministic")
	}
}
