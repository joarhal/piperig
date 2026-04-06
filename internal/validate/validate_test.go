package validate

import (
	"testing"

	"github.com/joarhal/piperig/internal/config"
	"github.com/joarhal/piperig/internal/pipe"
)

var cfg = config.Default()

func alwaysExists(_ string) bool { return true }
func neverExists(_ string) bool  { return false }

func TestValidPipe(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"src": "/data"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestRule2MissingJob(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/missing.py"},
		},
	}
	errs := Validate(p, cfg, neverExists, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for missing job file")
	}
	assertContains(t, errs, "file not found")
}

func TestRule3BadExtension(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.xyz"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for unsupported extension")
	}
	assertContains(t, errs, "unsupported extension")
}

func TestRule3NoExtensionDirectExec(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "mybinary"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("no-extension should be valid (direct exec), got %v", errs)
	}
}

func TestRule3CustomExtensionFromConfig(t *testing.T) {
	customCfg := config.Default()
	customCfg.Interpreters[".php"] = "php"
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.php"},
		},
	}
	errs := Validate(p, customCfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("custom extension should be valid, got %v", errs)
	}
}

func TestRule4NestedMissing(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "pipes/missing.pipe.yaml"},
		},
	}
	errs := Validate(p, cfg, neverExists, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for missing nested pipe")
	}
	assertContains(t, errs, "nested pipe not found")
}

func TestLoopOnNestedAllowed(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "pipes/child.pipe.yaml", Loop: map[string]any{"date": "-1d"}},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestEachOnNestedAllowed(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "pipes/child.pipe.yaml", Each: []pipe.StringMap{{"a": "1"}}},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestRule6BadInput(t *testing.T) {
	p := &pipe.Pipe{
		Input: "xml",
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid input mode")
	}
	assertContains(t, errs, "invalid input mode")
}

func TestRule6BadInputStep(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.py", Input: "xml"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	assertContains(t, errs, "invalid input mode")
}

func TestRule7BadTimeExpr(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"date": "-1x"},
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for bad time expression")
	}
	assertContains(t, errs, "cannot parse time expression")
}

func TestRule7NonTimeExprNotFlagged(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"size": "1920x1080", "name": "hello"},
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("non-time-expr values should not trigger errors, got %v", errs)
	}
}

func TestRule7BadTimeExprInLoop(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.py", Loop: map[string]any{"date": "-1x"}},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	assertContains(t, errs, "cannot parse time expression")
}

func TestRule8UnresolvedTemplate(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.py", With: pipe.StringMap{"out": "{missing}/file"}},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for unresolved template")
	}
	assertContains(t, errs, "template {missing} unresolved")
}

func TestRule8TemplateResolvedViaOverride(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.py", With: pipe.StringMap{"out": "{dest}/file"}},
		},
	}
	overrides := map[string]string{"dest": "/tmp"}
	errs := Validate(p, cfg, alwaysExists, overrides)
	if len(errs) != 0 {
		t.Errorf("template should resolve via override, got %v", errs)
	}
}

func TestRule8TemplateResolvedViaWith(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"base": "/data"},
		Steps: []pipe.Step{
			{Job: "scripts/run.py", With: pipe.StringMap{"out": "{base}/file"}},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("template should resolve via pipe with, got %v", errs)
	}
}

func TestRule8TemplateResolvedViaLoop(t *testing.T) {
	p := &pipe.Pipe{
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/run.py", With: pipe.StringMap{"out": "{date}.csv"}},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("template should resolve via loop key, got %v", errs)
	}
}

func TestMultipleErrors(t *testing.T) {
	p := &pipe.Pipe{
		Input: "xml",
		Steps: []pipe.Step{
			{Job: "scripts/run.xyz"},
			{Job: "scripts/missing.py"},
		},
	}
	errs := Validate(p, cfg, neverExists, nil)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestRule9BadRetryDelayPipe(t *testing.T) {
	p := &pipe.Pipe{
		RetryDelay: "5 seconds",
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	assertContains(t, errs, "invalid retry_delay")
}

func TestRule9BadTimeoutStep(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.py", Timeout: "10 minutes"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	assertContains(t, errs, "invalid timeout")
}

func TestRule9ValidDurations(t *testing.T) {
	p := &pipe.Pipe{
		RetryDelay: "5s",
		Timeout:    "10m",
		Steps: []pipe.Step{
			{Job: "scripts/run.py", RetryDelay: "500ms", Timeout: "1h"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid durations, got %v", errs)
	}
}

func TestRule12HookFileNotFound(t *testing.T) {
	p := &pipe.Pipe{
		OnFail: "scripts/missing.sh",
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	exists := func(path string) bool { return path != "scripts/missing.sh" }
	errs := Validate(p, cfg, exists, nil)
	assertContains(t, errs, "file not found")
}

func TestRule12HookBadExtension(t *testing.T) {
	p := &pipe.Pipe{
		OnSuccess: "scripts/hook.xyz",
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	assertContains(t, errs, "unsupported extension")
}

func TestRule12HookPipeYamlRejected(t *testing.T) {
	p := &pipe.Pipe{
		OnFail: "pipes/child.pipe.yaml",
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	assertContains(t, errs, "hooks cannot be .pipe.yaml")
}

func TestRule12StepHookValidation(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.py", OnFail: "scripts/missing.sh"},
		},
	}
	errs := Validate(p, cfg, func(path string) bool { return path != "scripts/missing.sh" }, nil)
	assertContains(t, errs, "file not found")
}

func TestRule12HookValid(t *testing.T) {
	p := &pipe.Pipe{
		OnFail:    "scripts/alert.sh",
		OnSuccess: "scripts/notify.py",
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid hooks, got %v", errs)
	}
}

func TestRule12HookNoExtensionDirectExec(t *testing.T) {
	p := &pipe.Pipe{
		OnFail: "bin/alert",
		Steps: []pipe.Step{
			{Job: "scripts/run.py"},
		},
	}
	errs := Validate(p, cfg, alwaysExists, nil)
	if len(errs) != 0 {
		t.Errorf("no-extension hook should be valid (direct exec), got %v", errs)
	}
}

func assertContains(t *testing.T, errs []error, substr string) {
	t.Helper()
	for _, err := range errs {
		if contains(err.Error(), substr) {
			return
		}
	}
	t.Errorf("expected error containing %q, got %v", substr, errs)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
