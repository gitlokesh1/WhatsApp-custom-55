package main

import "testing"

func TestWithdrawalAmounts(t *testing.T) {
	tests := []struct {
		name            string
		channel         withdrawalChannel
		gross, fee, net float64
	}{
		{"fixed", withdrawalChannel{FeeType: "fixed", FeeValue: 2.5}, 100, 2.5, 97.5},
		{"percent", withdrawalChannel{FeeType: "percent", FeeValue: 2.5}, 100, 2.5, 97.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fee, net := withdrawalAmounts(tt.channel, tt.gross)
			if fee != tt.fee || net != tt.net {
				t.Fatalf("got fee %.4f net %.4f", fee, net)
			}
		})
	}
}

func TestMaskBeneficiaryDoesNotExposeShortValues(t *testing.T) {
	masked := maskBeneficiary(map[string]string{"pin": "1234", "account": "123456789"})
	if masked["pin"] != "••••" {
		t.Fatalf("short value was not masked: %q", masked["pin"])
	}
	if masked["account"] != "•••••6789" {
		t.Fatalf("unexpected account mask: %q", masked["account"])
	}
}

func TestPayoutConfigured(t *testing.T) {
	t.Setenv("PAYOUT_GATEWAY_URL", "https://gateway.example/payout")
	t.Setenv("PAYOUT_GATEWAY_SECRET", "secret")
	t.Setenv("PAYOUT_DATA_ENCRYPTION_KEY", "encryption")
	if !payoutConfigured() {
		t.Fatal("expected payout configuration to be ready")
	}
	t.Setenv("PAYOUT_GATEWAY_SECRET", "")
	if payoutConfigured() {
		t.Fatal("expected missing secret to disable payouts")
	}
}
