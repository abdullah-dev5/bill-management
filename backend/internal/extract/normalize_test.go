package extract

import "testing"

func TestParseMoney(t *testing.T) {
	for in, want := range map[string]int64{
		"3266": 326600, "Rs. 3,266": 326600, "3,389.50": 338950, "(1,200)": -120000,
		"-45": -4500, "500 CR": -50000, "PKR 12,04,500": 120450000,
	} {
		if got, ok := ParseMoney(in); !ok || got != want {
			t.Errorf("ParseMoney(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	if _, ok := ParseMoney("n/a"); ok {
		t.Error("ParseMoney accepted text without digits")
	}
}

func TestParseDate(t *testing.T) {
	for in, want := range map[string]string{
		"05 OCT 26": "2026-10-05", "08-OCT-26": "2026-10-08", "18 Sep 2026": "2026-09-18",
		"Oct 5, 2026": "2026-10-05", "2026-10-05": "2026-10-05", "05/10/2026": "2026-10-05",
		"Due Date: 5-10-26": "2026-10-05", "September 18th, 2026": "2026-09-18",
	} {
		if got, ok := ParseDate(in); !ok || got != want {
			t.Errorf("ParseDate(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"31 FEB 26", "hello", "99/99/2026"} {
		if got, ok := ParseDate(bad); ok {
			t.Errorf("ParseDate(%q) = %q, want failure", bad, got)
		}
	}
}

func TestParseMonth(t *testing.T) {
	for in, want := range map[string]string{
		"SEP 26": "2026-09", "Sep-2026": "2026-09", "September 2026": "2026-09", "2026-09": "2026-09", "09/2026": "2026-09", "Sep25": "2025-09",
	} {
		if got, ok := ParseMonth(in); !ok || got != want {
			t.Errorf("ParseMonth(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestRedact(t *testing.T) {
	got := Redact("CNIC 35202-1234567-1 or 3520212345671. Name: Ali Raza Khan", "Ali Raza Khan")
	if got != "CNIC [redacted] or [redacted]. Name: [redacted]" {
		t.Errorf("Redact = %q", got)
	}
}
