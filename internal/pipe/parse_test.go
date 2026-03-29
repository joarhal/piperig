package pipe

import (
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", "pipes", name)
}

func TestLoadMinimal(t *testing.T) {
	p, err := Load(testdataPath("minimal.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("steps: got %d, want 1", len(p.Steps))
	}
	if p.Steps[0].Job != "scripts/example.py" {
		t.Errorf("job: got %q, want %q", p.Steps[0].Job, "scripts/example.py")
	}
	if p.With != nil {
		t.Errorf("with: got %v, want nil", p.With)
	}
	if p.Loop != nil {
		t.Errorf("loop: got %v, want nil", p.Loop)
	}
	if p.Each != nil {
		t.Errorf("each: got %v, want nil", p.Each)
	}
	if p.Retry != nil {
		t.Errorf("retry: got %v, want nil", p.Retry)
	}
	if p.Description != "" {
		t.Errorf("description: got %q, want empty", p.Description)
	}
}

func TestLoadWithNormalization(t *testing.T) {
	p, err := Load(testdataPath("with_only.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if p.Description != "Test pipe with params" {
		t.Errorf("description: got %q", p.Description)
	}

	cases := map[string]string{
		"src":     "/data/photos",
		"quality": "80",
		"enabled": "true",
		"ratio":   "0.5",
	}
	for k, want := range cases {
		if got := p.With[k]; got != want {
			t.Errorf("with[%s]: got %q, want %q", k, got, want)
		}
	}

	wantLog := []string{"label", "file"}
	if len(p.Log) != len(wantLog) {
		t.Fatalf("log: got %v, want %v", p.Log, wantLog)
	}
	for i, v := range wantLog {
		if p.Log[i] != v {
			t.Errorf("log[%d]: got %q, want %q", i, p.Log[i], v)
		}
	}
}

func TestLoadEach(t *testing.T) {
	p, err := Load(testdataPath("each.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Each) != 3 {
		t.Fatalf("each: got %d items, want 3", len(p.Each))
	}
	if p.Each[0]["size"] != "1920x1080" {
		t.Errorf("each[0].size: got %q", p.Each[0]["size"])
	}
	if p.Each[0]["label"] != "fullhd" {
		t.Errorf("each[0].label: got %q", p.Each[0]["label"])
	}
	if p.Each[2]["label"] != "thumb" {
		t.Errorf("each[2].label: got %q", p.Each[2]["label"])
	}
}

func TestLoadEachFalse(t *testing.T) {
	p, err := Load(testdataPath("each_false.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Each) != 2 {
		t.Fatalf("pipe each: got %d items, want 2", len(p.Each))
	}
	step := p.Steps[0]
	if !step.EachOff {
		t.Error("step.EachOff: got false, want true")
	}
	if step.Each != nil {
		t.Errorf("step.Each: got %v, want nil", step.Each)
	}
}

func TestLoadLoopFalse(t *testing.T) {
	p, err := Load(testdataPath("loop_false.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Loop == nil {
		t.Fatal("pipe loop: got nil, want non-nil")
	}
	if p.Loop["date"] != "-2d..-1d" {
		t.Errorf("pipe loop.date: got %v", p.Loop["date"])
	}
	step := p.Steps[0]
	if !step.LoopOff {
		t.Error("step.LoopOff: got false, want true")
	}
	if step.Loop != nil {
		t.Errorf("step.Loop: got %v, want nil", step.Loop)
	}
}

func TestLoadRetry(t *testing.T) {
	p, err := Load(testdataPath("retry.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if p.Retry == nil || *p.Retry != 3 {
		t.Errorf("pipe retry: got %v, want 3", p.Retry)
	}
	if p.RetryDelay != "5s" {
		t.Errorf("pipe retry_delay: got %q, want %q", p.RetryDelay, "5s")
	}

	// step 0: retry: 5
	s0 := p.Steps[0]
	if s0.Retry == nil || *s0.Retry != 5 {
		t.Errorf("step[0] retry: got %v, want 5", s0.Retry)
	}
	if s0.RetryOff {
		t.Error("step[0] RetryOff: got true, want false")
	}

	// step 1: retry: false
	s1 := p.Steps[1]
	if !s1.RetryOff {
		t.Error("step[1] RetryOff: got false, want true")
	}
	if s1.Retry != nil {
		t.Errorf("step[1] retry: got %v, want nil", s1.Retry)
	}
}

func TestLoadInputModes(t *testing.T) {
	p, err := Load(testdataPath("input_json.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Input != InputJSON {
		t.Errorf("pipe input: got %q, want %q", p.Input, InputJSON)
	}
	if p.Steps[0].Input != InputArgs {
		t.Errorf("step[0] input: got %q, want %q", p.Steps[0].Input, InputArgs)
	}
	if p.Steps[1].Input != "" {
		t.Errorf("step[1] input: got %q, want empty (inherit)", p.Steps[1].Input)
	}
}

func TestLoadUnknownPipeKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pipe.yaml")
	if err := writeFile(path, "foo: bar\nsteps:\n  - job: scripts/example.py\n"); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown pipe key")
	}
	if !strings.Contains(err.Error(), "unknown key") {
		t.Errorf("error %q should contain %q", err, "unknown key")
	}
}

func TestLoadUnknownStepKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pipe.yaml")
	if err := writeFile(path, "steps:\n  - job: scripts/example.py\n    bar: baz\n"); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown step key")
	}
	if !strings.Contains(err.Error(), "unknown key") {
		t.Errorf("error %q should contain %q", err, "unknown key")
	}
}

func TestLoadNestedObjectInWith(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pipe.yaml")
	if err := writeFile(path, "with:\n  config:\n    nested: obj\nsteps:\n  - job: scripts/example.py\n"); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for nested object in with")
	}
	if !strings.Contains(err.Error(), "nested objects not allowed") {
		t.Errorf("error %q should contain %q", err, "nested objects not allowed")
	}
}

func TestLoadListInWith(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pipe.yaml")
	if err := writeFile(path, "with:\n  items:\n    - a\n    - b\nsteps:\n  - job: scripts/example.py\n"); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for list in with")
	}
	if !strings.Contains(err.Error(), "lists not allowed") {
		t.Errorf("error %q should contain %q", err, "lists not allowed")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("testdata/pipes/nonexistent.pipe.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadStepLog(t *testing.T) {
	p, err := Load(testdataPath("step_log.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Pipe-level log
	if len(p.Log) != 2 || p.Log[0] != "label" || p.Log[1] != "file" {
		t.Errorf("pipe log = %v, want [label file]", p.Log)
	}
	// Step 0: no step-level log
	if p.Steps[0].Log != nil {
		t.Errorf("step 0 log = %v, want nil", p.Steps[0].Log)
	}
	// Step 1: step-level log overrides
	if len(p.Steps[1].Log) != 3 || p.Steps[1].Log[0] != "file" {
		t.Errorf("step 1 log = %v, want [file status url]", p.Steps[1].Log)
	}
}

func TestLoadHidden(t *testing.T) {
	p, err := Load(testdataPath("hidden.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Hidden {
		t.Error("Hidden: got false, want true")
	}
	if p.Description != "Helper pipe" {
		t.Errorf("Description: got %q, want %q", p.Description, "Helper pipe")
	}
}

func TestLoadHiddenDefault(t *testing.T) {
	p, err := Load(testdataPath("minimal.pipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Hidden {
		t.Error("Hidden: got true, want false (default)")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pipe.yaml")
	if err := writeFile(path, "{{invalid yaml"); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
