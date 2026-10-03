package httpapi

import (
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/store"
)

func TestMatchProvider(t *testing.T) {
	hints := []store.ListProviderHintsRow{
		{ShortName: "HESCO", Name: "Hyderabad Electric Supply Company"},
		{ShortName: "LESCO", Name: "Lahore Electric Supply Company"},
		{ShortName: "SNGPL", Name: "Sui Northern Gas Pipelines"},
	}
	for in, want := range map[string]string{
		"HESCO": "HESCO", "hyderabad electric supply company": "HESCO", "LESCO Lahore": "LESCO",
		"Sui Northern Gas": "SNGPL", "Unknown Telecom": "",
	} {
		h, ok := matchProvider(hints, in)
		if got := h.ShortName; ok != (want != "") || got != want {
			t.Errorf("matchProvider(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}
