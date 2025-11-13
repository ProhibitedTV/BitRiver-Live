package models

import (
	"encoding/json"
	"testing"
)

func TestParseMoneyValid(t *testing.T) {
	cases := []struct {
		name  string
		input string
		units int64
	}{
		{name: "zero", input: "0", units: 0},
		{name: "integer", input: "42", units: 4200000000},
		{name: "fraction", input: "5.5", units: 550000000},
		{name: "maxFraction", input: "0.12345678", units: 12345678},
		{name: "negative", input: "-1.25", units: -125000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			money, err := ParseMoney(tc.input)
			if err != nil {
				t.Fatalf("ParseMoney(%q) returned error: %v", tc.input, err)
			}
			if money.MinorUnits() != tc.units {
				t.Fatalf("expected %d minor units, got %d", tc.units, money.MinorUnits())
			}
			if got := money.DecimalString(); got != tc.input {
				t.Fatalf("DecimalString mismatch: want %q, got %q", tc.input, got)
			}
		})
	}
}

func TestParseMoneyInvalid(t *testing.T) {
	inputs := []string{"", "abc", "1.000000001", "0.123456789"}
	for _, input := range inputs {
		if _, err := ParseMoney(input); err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
}

func TestMoneyJSONRoundTrip(t *testing.T) {
	original := MustParseMoney("12.34000001")
	payload, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(payload) != "12.34000001" {
		t.Fatalf("expected canonical JSON number, got %s", payload)
	}
	var decoded Money
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MinorUnits() != original.MinorUnits() {
		t.Fatalf("expected %d, got %d", original.MinorUnits(), decoded.MinorUnits())
	}
}

func TestMoneyAdd(t *testing.T) {
	first := MustParseMoney("1.1")
	second := MustParseMoney("2.25")
	sum := first.Add(second)
	if sum.MinorUnits() != 335000000 {
		t.Fatalf("expected 3.35 units, got %d", sum.MinorUnits())
	}
	if sum.DecimalString() != "3.35" {
		t.Fatalf("expected decimal string 3.35, got %q", sum.DecimalString())
	}
}
