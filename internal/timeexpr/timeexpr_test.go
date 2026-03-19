package timeexpr

import (
	"testing"
	"time"
)

var now = time.Date(2026, 3, 19, 11, 43, 25, 0, time.UTC)

func TestResolve(t *testing.T) {
	tests := []struct {
		expr string
		now  time.Time
		want string
	}{
		{"-1d", now, "2026-03-18"},
		{"0d", now, "2026-03-19"},
		{"1d", now, "2026-03-20"},
		{"+1d", now, "2026-03-20"},
		{"-2h", now, "2026-03-19T09:00:00"},
		{"0h", now, "2026-03-19T11:00:00"},
		{"-30m", now, "2026-03-19T11:13:00"},
		{"-10s", now, "2026-03-19T11:43:15"},
		{"-1w", now, "2026-03-09"},
		{"0w", now, "2026-03-16"},
		// month boundary
		{"-1d", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), "2026-02-28"},
		// year boundary
		{"-1d", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "2025-12-31"},
		// leap year
		{"-1d", time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), "2024-02-29"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := Resolve(tt.expr, tt.now)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tt.expr, err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

func TestResolveError(t *testing.T) {
	_, err := Resolve("hello", now)
	if err == nil {
		t.Error("expected error for invalid expression")
	}
}

func TestExpandRange(t *testing.T) {
	tests := []struct {
		expr    string
		wantLen int
	}{
		{"-2d..-1d", 2},
		{"-1d..-1d", 1},
		{"-24h..-1h", 24},
		{"-4w..-1w", 4},
		{"2026-03-01..2026-03-03", 3},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := ExpandRange(tt.expr, now)
			if err != nil {
				t.Fatalf("ExpandRange(%q): %v", tt.expr, err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("ExpandRange(%q) = %d items %v, want %d", tt.expr, len(got), got, tt.wantLen)
			}
		})
	}
}

func TestExpandRangeValues(t *testing.T) {
	got, err := ExpandRange("-2d..-1d", now)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "2026-03-17" || got[1] != "2026-03-18" {
		t.Errorf("got %v, want [2026-03-17 2026-03-18]", got)
	}
}

func TestExpandRangeAbsolute(t *testing.T) {
	got, err := ExpandRange("2026-03-01..2026-03-03", now)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-03-01", "2026-03-02", "2026-03-03"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpandRangeInverted(t *testing.T) {
	_, err := ExpandRange("-1d..-3d", now)
	if err == nil {
		t.Error("expected error for inverted range")
	}
}

func TestIsTimeExpr(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"-1d", true},
		{"0h", true},
		{"+2w", true},
		{"hello", false},
		{"1920x1080", false},
		{"123", false},
		{"", false},
		{"-1x", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsTimeExpr(tt.input); got != tt.want {
				t.Errorf("IsTimeExpr(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsRange(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"-2d..-1d", true},
		{"2026-03-01..2026-03-03", true},
		{"-1d", false},
		{"hello", false},
		{"a..b", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsRange(tt.input); got != tt.want {
				t.Errorf("IsRange(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
