package main

import "testing"

func TestValidateDriver(t *testing.T) {
	tests := []struct {
		name      string
		driver    string
		isStub    bool
		allowStub bool
		wantErr   bool
	}{
		{name: "json with stub", driver: "json", isStub: true, wantErr: false},
		{name: "postgres with real", driver: "postgres", isStub: false, wantErr: false},
		{name: "postgres with stub denied", driver: "postgres", isStub: true, allowStub: false, wantErr: true},
		{name: "postgres with stub allowed", driver: "postgres", isStub: true, allowStub: true, wantErr: false},
		{name: "invalid driver", driver: "sqlite", isStub: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDriver(tt.driver, tt.isStub, tt.allowStub)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDriver() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
