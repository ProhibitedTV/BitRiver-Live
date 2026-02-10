package main

import "testing"

func TestValidateDriver(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		wantErr bool
	}{
		{name: "postgres allowed", driver: "postgres", wantErr: false},
		{name: "invalid driver rejected", driver: "sqlite", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDriver(tc.driver)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
