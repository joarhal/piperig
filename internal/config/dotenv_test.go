package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv_Basic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\nBAZ=qux\n"), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	env, err := loadDotEnv()
	if err != nil {
		t.Fatal(err)
	}
	if env["FOO"] != "bar" {
		t.Errorf("FOO = %q, want bar", env["FOO"])
	}
	if env["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want qux", env["BAZ"])
	}
}

func TestLoadDotEnv_Quotes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("A=\"hello world\"\nB='single quoted'\n"), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	env, err := loadDotEnv()
	if err != nil {
		t.Fatal(err)
	}
	if env["A"] != "hello world" {
		t.Errorf("A = %q, want 'hello world'", env["A"])
	}
	if env["B"] != "single quoted" {
		t.Errorf("B = %q, want 'single quoted'", env["B"])
	}
}

func TestLoadDotEnv_CommentsAndEmpty(t *testing.T) {
	content := "# this is a comment\n\nKEY=value\n\n# another comment\n"
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	env, err := loadDotEnv()
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 1 {
		t.Errorf("got %d keys, want 1: %v", len(env), env)
	}
	if env["KEY"] != "value" {
		t.Errorf("KEY = %q, want value", env["KEY"])
	}
}

func TestLoadDotEnv_ExportPrefix(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("export MY_VAR=hello\n"), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	env, err := loadDotEnv()
	if err != nil {
		t.Fatal(err)
	}
	if env["MY_VAR"] != "hello" {
		t.Errorf("MY_VAR = %q, want hello", env["MY_VAR"])
	}
}

func TestLoadDotEnv_NoFile(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	env, err := loadDotEnv()
	if err != nil {
		t.Fatal(err)
	}
	if env != nil {
		t.Errorf("expected nil, got %v", env)
	}
}

func TestLoadDotEnv_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("EMPTY=\n"), 0o644)

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	env, err := loadDotEnv()
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := env["EMPTY"]; !ok || v != "" {
		t.Errorf("EMPTY = %q (ok=%v), want empty string", v, ok)
	}
}

func TestApplyDotEnv_NoOverride(t *testing.T) {
	key := "PIPERIG_TEST_APPLY_" + t.Name()
	os.Setenv(key, "original")
	defer os.Unsetenv(key)

	applyDotEnv(map[string]string{key: "from_dotenv"})

	if got := os.Getenv(key); got != "original" {
		t.Errorf("expected system value 'original', got %q", got)
	}
}

func TestApplyDotEnv_SetsUnset(t *testing.T) {
	key := "PIPERIG_TEST_UNSET_" + t.Name()
	os.Unsetenv(key)
	defer os.Unsetenv(key)

	applyDotEnv(map[string]string{key: "from_dotenv"})

	if got := os.Getenv(key); got != "from_dotenv" {
		t.Errorf("expected 'from_dotenv', got %q", got)
	}
}

func TestStripQuotes(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`"`, `"`},
		{`""`, ""},
		{`"mismatched'`, `"mismatched'`},
	}
	for _, tt := range tests {
		if got := stripQuotes(tt.in); got != tt.want {
			t.Errorf("stripQuotes(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
