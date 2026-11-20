package config

import "testing"

func baseEnvironment() map[string]string {
	return map[string]string{
		"DATABASE_URL":  "postgres://wgswp:change-me@localhost:5432/wgswp?sslmode=disable",
		"STEAM_API_KEY": "test-key",
	}
}

func TestParseTrustedProxyHops(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "unset defaults to zero", value: "", want: 0},
		{name: "single proxy", value: "1", want: 1},
		{name: "chained proxies", value: "3", want: 3},
		{name: "zero", value: "0", want: 0},
		{name: "negative", value: "-1", wantErr: true},
		{name: "not a number", value: "nginx", wantErr: true},
		{name: "fractional", value: "1.5", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := baseEnvironment()
			if test.value != "" {
				environment["TRUSTED_PROXY_HOPS"] = test.value
			}

			parsed, err := Parse(environment, t.TempDir())
			if test.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) succeeded, want error", test.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", test.value, err)
			}
			if parsed.TrustedProxyHops != test.want {
				t.Fatalf("TrustedProxyHops = %d, want %d", parsed.TrustedProxyHops, test.want)
			}
		})
	}
}
