package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSchedule(t *testing.T) {
	dir := t.TempDir()
	content := `- name: daily
  cron: "0 5 * * *"
  run:
    - pipes/daily/
  with:
    quality: "80"

- name: healthcheck
  every: 10m
  run:
    - pipes/health.pipe.yaml
`
	path := filepath.Join(dir, "schedule.yaml")
	os.WriteFile(path, []byte(content), 0o644)

	entries, err := LoadSchedule(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Name != "daily" {
		t.Errorf("name = %q", entries[0].Name)
	}
	if entries[0].Cron != "0 5 * * *" {
		t.Errorf("cron = %q", entries[0].Cron)
	}
	if entries[0].With["quality"] != "80" {
		t.Errorf("with.quality = %q", entries[0].With["quality"])
	}
	if entries[1].Every != "10m" {
		t.Errorf("every = %q", entries[1].Every)
	}
}

func TestValidateCronXorEvery(t *testing.T) {
	tests := []struct {
		name    string
		entry   Entry
		wantErr bool
	}{
		{"cron only", Entry{Name: "a", Cron: "0 5 * * *", Run: []string{"x"}}, false},
		{"every only", Entry{Name: "b", Every: "10m", Run: []string{"x"}}, false},
		{"both", Entry{Name: "c", Cron: "0 5 * * *", Every: "10m", Run: []string{"x"}}, true},
		{"neither", Entry{Name: "d", Run: []string{"x"}}, true},
		{"empty run", Entry{Name: "e", Cron: "0 5 * * *"}, true},
		{"bad cron", Entry{Name: "f", Cron: "bad", Run: []string{"x"}}, true},
		{"bad every", Entry{Name: "g", Every: "bad", Run: []string{"x"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateEntries([]Entry{tt.entry})
			if tt.wantErr && len(errs) == 0 {
				t.Error("expected error")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}
