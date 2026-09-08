package main

import "testing"

func TestLevelForBoundaries(t *testing.T) {
	tests := []struct {
		total int
		name  string
		floor int
		next  int
	}{
		{0, "Bronze", 0, 25}, {24, "Bronze", 0, 25},
		{25, "Silver", 25, 100}, {99, "Silver", 25, 100},
		{100, "Gold", 100, 250}, {249, "Gold", 100, 250},
		{250, "Platinum", 250, 1000}, {999, "Platinum", 250, 1000},
		{1000, "Diamond", 1000, 1000},
	}
	for _, test := range tests {
		name, floor, next := levelFor(test.total)
		if name != test.name || floor != test.floor || next != test.next {
			t.Fatalf("levelFor(%d) = %q,%d,%d; want %q,%d,%d", test.total, name, floor, next, test.name, test.floor, test.next)
		}
	}
}

func TestValidateCTAURLAllowsOnlyInternalPaths(t *testing.T) {
	for _, value := range []string{"", "/tasks", "/referrals?from=banner"} {
		if !validateCTAURL(value) {
			t.Fatalf("expected %q to be allowed", value)
		}
	}
	for _, value := range []string{"https://example.com", "//example.com", "javascript:alert(1)", "tasks"} {
		if validateCTAURL(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestValidateTimezone(t *testing.T) {
	if !validateTimezone("Asia/Kolkata") {
		t.Fatal("expected a valid IANA timezone")
	}
	if validateTimezone("Not/A_Zone") {
		t.Fatal("expected an invalid timezone to be rejected")
	}
}
