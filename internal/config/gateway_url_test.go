package config

import "testing"

func TestNormalizeGatewayURL(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"", ""},
		{"   ", ""},
		{" https://gateway.example.com/ ", "https://gateway.example.com"},
		{"http://gateway.example.com", "http://gateway.example.com"},
		{"https://gateway.example.com:8443/relay///", "https://gateway.example.com:8443/relay"},
		{"http://127.0.0.1:8787/", "http://127.0.0.1:8787"},
		{"http://[::1]:8787/relay/", "http://[::1]:8787/relay"},
		{"https://gateway.example.com/a%2Fb/", "https://gateway.example.com/a%2Fb"},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := NormalizeGatewayURL(test.input)
			if err != nil || got != test.want {
				t.Fatalf("NormalizeGatewayURL(%q) = %q, %v; want %q", test.input, got, err, test.want)
			}
		})
	}
}

func TestNormalizeGatewayURLRejectsInvalidAddresses(t *testing.T) {
	for _, input := range []string{
		"gateway.example.com", "//gateway.example.com", "https://", "https:///relay",
		"ftp://gateway.example.com", "javascript:alert(1)", "https://user:password@gateway.example.com",
		"https://gateway.example.com?key=value", "https://gateway.example.com#anchor",
		"https://gateway.example.com?", "https://gateway.example.com#",
		"https://bad host", "https://gateway.example.com/bad path", `https://gateway.example.com\relay`,
		"https://gateway.example.com:0", "https://gateway.example.com:65536",
		"https://gateway.example.com:", "https://gateway.example.com:abc",
	} {
		t.Run(input, func(t *testing.T) {
			if got, err := NormalizeGatewayURL(input); err == nil {
				t.Fatalf("accepted invalid URL %q as %q", input, got)
			}
		})
	}
}
