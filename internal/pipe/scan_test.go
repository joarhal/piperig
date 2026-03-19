package pipe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestScanMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(filepath.Join(dir, "b.pipe.yaml"), "steps: []")
	writeFile(filepath.Join(dir, "a.pipe.yaml"), "steps: []")

	paths, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("got %d files, want 2", len(paths))
	}
	if !strings.HasSuffix(paths[0], "a.pipe.yaml") {
		t.Errorf("first file: got %s, want a.pipe.yaml", paths[0])
	}
	if !strings.HasSuffix(paths[1], "b.pipe.yaml") {
		t.Errorf("second file: got %s, want b.pipe.yaml", paths[1])
	}
}

func TestScanRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(filepath.Join(dir, "root.pipe.yaml"), "steps: []")
	writeFile(filepath.Join(dir, "sub", "nested.pipe.yaml"), "steps: []")
	writeFile(filepath.Join(dir, "deep", "sub", "deep.pipe.yaml"), "steps: []")

	paths, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("got %d files, want 3", len(paths))
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("path not absolute: %s", p)
		}
	}
}

func TestScanIgnoresNonPipeYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(filepath.Join(dir, "config.yaml"), "key: value")
	writeFile(filepath.Join(dir, "docker-compose.yaml"), "version: 3")
	writeFile(filepath.Join(dir, "real.pipe.yaml"), "steps: []")

	paths, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("got %d files, want 1", len(paths))
	}
	if !strings.HasSuffix(paths[0], "real.pipe.yaml") {
		t.Errorf("got %s, want real.pipe.yaml", paths[0])
	}
}

func TestScanEmptyDir(t *testing.T) {
	dir := t.TempDir()
	paths, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Errorf("got %d files, want 0", len(paths))
	}
}

func TestScanFileInsteadOfDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	writeFile(file, "hello")

	_, err := Scan(file)
	if err == nil {
		t.Error("expected error for file instead of directory")
	}
}

func TestScanAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(filepath.Join(dir, "test.pipe.yaml"), "steps: []")

	paths, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("expected absolute path, got %s", p)
		}
	}
}
