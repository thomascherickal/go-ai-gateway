package redact

import "testing"

func TestValidatorsEdgeCases(t *testing.T) {
	for _, bad := range []string{"12-34", "000-11-2222", "666-11-2222", "900-11-2222", "123-00-2222", "123-45-0000"} {
		if isValidSSN(bad) {
			t.Fatalf("expected invalid SSN: %q", bad)
		}
	}
	for _, bad := range []string{"1.2.3", "01.2.3.4", "256.1.1.1", "a.b.c.d", ".1.2.3"} {
		if isValidIPv4(bad) {
			t.Fatalf("expected invalid IPv4: %q", bad)
		}
	}
	s := NewSession()
	if got := s.RedactText(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
