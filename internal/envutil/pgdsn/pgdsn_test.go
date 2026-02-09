package pgdsn

import "testing"

func TestSSLModeDisable(t *testing.T) {
	if !SSLModeDisable("postgres://user:pass@db:5432/app?sslmode=disable") {
		t.Fatal("expected sslmode=disable to be detected")
	}
	if SSLModeDisable("postgres://user:pass@db:5432/app?sslmode=require") {
		t.Fatal("did not expect sslmode=require to be treated as disable")
	}
}

func TestIsComposePostgresDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "keyword host", dsn: "host=postgres user=app password=secret sslmode=disable", want: true},
		{name: "url host", dsn: "postgres://user:pass@postgres:5432/app?sslmode=disable", want: true},
		{name: "external", dsn: "postgres://user:pass@db.example:5432/app?sslmode=disable", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsComposePostgresDSN(tc.dsn); got != tc.want {
				t.Fatalf("IsComposePostgresDSN(%q) = %v, want %v", tc.dsn, got, tc.want)
			}
		})
	}
}

func TestValidateTLSPolicy(t *testing.T) {
	if err := ValidateTLSPolicy("postgres://user:pass@db.example:5432/app?sslmode=disable", "BITRIVER_LIVE_POSTGRES_DSN"); err == nil {
		t.Fatal("expected external sslmode=disable to fail")
	}
	if err := ValidateTLSPolicy("postgres://user:pass@postgres:5432/app?sslmode=disable", "BITRIVER_LIVE_POSTGRES_DSN"); err != nil {
		t.Fatalf("expected compose DSN to pass, got %v", err)
	}
}
