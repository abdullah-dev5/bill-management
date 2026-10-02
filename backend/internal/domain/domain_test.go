package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestValidPeriodKey(t *testing.T) {
	for in, want := range map[string]bool{
		"2026-09": true, "2026-12": true, "2026-01": true,
		"2026-13": false, "2026-00": false, "26-09": false, "2026-9": false, "": false,
	} {
		if got := ValidPeriodKey(in); got != want {
			t.Errorf("ValidPeriodKey(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMonthBounds(t *testing.T) {
	start, end, err := MonthBounds("2026-12")
	if err != nil {
		t.Fatal(err)
	}
	if start.Format("2006-01-02") != "2026-12-01" || end.Format("2006-01-02") != "2027-01-01" {
		t.Fatalf("got %s – %s", start, end)
	}
}

func TestTodayUsesHouseholdTimezone(t *testing.T) {
	karachi, err := time.LoadLocation("Asia/Karachi") // UTC+5
	if err != nil {
		t.Fatal(err)
	}
	// 20:00 UTC on 30 Sep is already 01:00 on 1 Oct in Karachi.
	now := time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)
	if got := Today(now, karachi).Format("2006-01-02"); got != "2026-10-01" {
		t.Fatalf("Today = %s, want 2026-10-01", got)
	}
}

func TestNormalizeReference(t *testing.T) {
	if got := NormalizeReference(" 04 11132-1234567 "); got != "04111321234567" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeReference("ab-12"); got != "AB12" {
		t.Fatalf("got %q", got)
	}
}

func TestMatchesPattern(t *testing.T) {
	ok, err := MatchesPattern("04111321234567", `^\d{14}$`)
	if err != nil || !ok {
		t.Fatalf("expected match, got %v %v", ok, err)
	}
	ok, _ = MatchesPattern("123", `^\d{14}$`)
	if ok {
		t.Fatal("expected no match")
	}
	ok, _ = MatchesPattern("anything", "")
	if !ok {
		t.Fatal("empty pattern must accept")
	}
}

func TestNormalizeEmail(t *testing.T) {
	if e, ok := NormalizeEmail("  Ali@Example.COM "); !ok || e != "ali@example.com" {
		t.Fatalf("got %q %v", e, ok)
	}
	if _, ok := NormalizeEmail("not-an-email"); ok {
		t.Fatal("expected invalid")
	}
}

func TestCleanTags(t *testing.T) {
	got := CleanTags([]string{" Kitchen ", "kitchen", "", "AC"})
	if !reflect.DeepEqual(got, []string{"Kitchen", "AC"}) {
		t.Fatalf("got %v", got)
	}
}
