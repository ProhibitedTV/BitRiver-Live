package main

import "testing"

func TestValidateDriver(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		isStub  bool
		wantErr bool
	}{
		{name: "postgres real allowed", driver: "postgres", isStub: false, wantErr: false},
		{name: "postgres stub rejected", driver: "postgres", isStub: true, wantErr: true},
		{name: "invalid driver rejected", driver: "sqlite", isStub: false, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDriver(tc.driver, tc.isStub)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
