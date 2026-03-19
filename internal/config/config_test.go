package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	expected := map[string]string{
		".py": "python",
		".sh": "bash",
		".js": "node",
		".ts": "npx tsx",
		".rb": "ruby",
	}
	for ext, want := range expected {
		if got := cfg.Interpreters[ext]; got != want {
			t.Errorf("Interpreters[%s] = %q, want %q", ext, got, want)
		}
	}
	if len(cfg.Interpreters) != len(expected) {
		t.Errorf("got %d interpreters, want %d", len(cfg.Interpreters), len(expected))
	}
}

func TestLoadNoFile(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interpreters[".py"] != "python" {
		t.Errorf("expected default .py=python, got %q", cfg.Interpreters[".py"])
	}
}

func TestLoadOverride(t *testing.T) {
	dir := t.TempDir()
	content := "interpreters:\n  .py: python3.11\n"
	os.WriteFile(filepath.Join(dir, ".piperig.yaml"), []byte(content), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interpreters[".py"] != "python3.11" {
		t.Errorf(".py = %q, want python3.11", cfg.Interpreters[".py"])
	}
	// Defaults still present
	if cfg.Interpreters[".sh"] != "bash" {
		t.Errorf(".sh = %q, want bash", cfg.Interpreters[".sh"])
	}
}

func TestLoadNewExtension(t *testing.T) {
	dir := t.TempDir()
	content := "interpreters:\n  .php: php\n  .lua: lua\n"
	os.WriteFile(filepath.Join(dir, ".piperig.yaml"), []byte(content), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interpreters[".php"] != "php" {
		t.Errorf(".php = %q, want php", cfg.Interpreters[".php"])
	}
	if cfg.Interpreters[".lua"] != "lua" {
		t.Errorf(".lua = %q, want lua", cfg.Interpreters[".lua"])
	}
}

func TestLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".piperig.yaml"), []byte("{{bad"), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	_, err := Load()
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}
