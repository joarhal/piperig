package expand

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joarhal/piperig/internal/pipe"
)

var now = time.Date(2026, 3, 19, 11, 43, 25, 0, time.UTC)

func intPtr(n int) *int { return &n }

func TestWithOnly(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"src": "/data", "quality": "80"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("steps: got %d, want 1", len(plan.Steps))
	}
	if len(plan.Steps[0].Calls) != 1 {
		t.Fatalf("calls: got %d, want 1", len(plan.Steps[0].Calls))
	}
	call := plan.Steps[0].Calls[0]
	if call.Params["src"] != "/data" {
		t.Errorf("src = %q", call.Params["src"])
	}
	if call.Params["quality"] != "80" {
		t.Errorf("quality = %q", call.Params["quality"])
	}
}

func TestStepWithOverridesPipeWith(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"quality": "80"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py", With: pipe.StringMap{"quality": "100"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Params["quality"] != "100" {
		t.Errorf("quality = %q, want 100", plan.Steps[0].Calls[0].Params["quality"])
	}
}

func TestLoopTimeRange(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/download.sh", Loop: map[string]any{"date": "-2d..-1d"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	calls := plan.Steps[0].Calls
	if len(calls) != 2 {
		t.Fatalf("calls: got %d, want 2", len(calls))
	}
	if calls[0].Params["date"] != "2026-03-17" {
		t.Errorf("call[0].date = %q", calls[0].Params["date"])
	}
	if calls[1].Params["date"] != "2026-03-18" {
		t.Errorf("call[1].date = %q", calls[1].Params["date"])
	}
}

func TestLoopNumericRange(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{"n": "1..3"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	calls := plan.Steps[0].Calls
	if len(calls) != 3 {
		t.Fatalf("calls: got %d, want 3", len(calls))
	}
	for i, want := range []string{"1", "2", "3"} {
		if calls[i].Params["n"] != want {
			t.Errorf("call[%d].n = %q, want %q", i, calls[i].Params["n"], want)
		}
	}
}

func TestLoop_InvertedNumericRange(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{"n": "5..1"}},
		},
	}
	_, err := Expand(p, nil, now)
	if err == nil {
		t.Fatal("expected error for inverted numeric range")
	}
	if !strings.Contains(err.Error(), "inverted numeric range") {
		t.Errorf("error = %q, want it to contain 'inverted numeric range'", err)
	}
}

func TestLoopExplicitList(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/deploy.sh", Loop: map[string]any{"region": []any{"eu", "us"}}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps[0].Calls) != 2 {
		t.Fatalf("calls: got %d, want 2", len(plan.Steps[0].Calls))
	}
}

func TestLoopMultiKeyCartesian(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{
				"date":   "-2d..-1d",
				"region": []any{"eu", "us"},
			}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// 2 dates × 2 regions = 4
	if len(plan.Steps[0].Calls) != 4 {
		t.Fatalf("calls: got %d, want 4", len(plan.Steps[0].Calls))
	}
}

func TestEachBasic(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"a": "1"},
			{"a": "2"},
		},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	calls := plan.Steps[0].Calls
	if len(calls) != 2 {
		t.Fatalf("calls: got %d, want 2", len(calls))
	}
	if calls[0].Params["a"] != "1" {
		t.Errorf("call[0].a = %q", calls[0].Params["a"])
	}
}

func TestEachSparseKeys(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"a": "1", "b": "2"},
			{"a": "3"},
		},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	calls := plan.Steps[0].Calls
	if calls[0].Params["b"] != "2" {
		t.Errorf("call[0].b = %q, want 2", calls[0].Params["b"])
	}
	if _, ok := calls[1].Params["b"]; ok {
		t.Errorf("call[1] should not have key b, got %q", calls[1].Params["b"])
	}
}

func TestEachTimesLoop(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"size": "1920x1080"},
			{"size": "128x128"},
		},
		Loop: map[string]any{"date": "-3d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// 2 each × 3 dates = 6
	if len(plan.Steps[0].Calls) != 6 {
		t.Fatalf("calls: got %d, want 6", len(plan.Steps[0].Calls))
	}
}

func TestEachFalse(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"size": "1920x1080"},
			{"size": "128x128"},
		},
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/download.sh", EachOff: true},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// No each, 2 dates = 2
	if len(plan.Steps[0].Calls) != 2 {
		t.Fatalf("calls: got %d, want 2", len(plan.Steps[0].Calls))
	}
}

func TestLoopFalse(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"size": "1920x1080"},
			{"size": "128x128"},
		},
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py", LoopOff: true},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// 2 each, no loop = 2
	if len(plan.Steps[0].Calls) != 2 {
		t.Fatalf("calls: got %d, want 2", len(plan.Steps[0].Calls))
	}
}

func TestBothOff(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{{"size": "1920x1080"}},
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/upload.sh", EachOff: true, LoopOff: true},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps[0].Calls) != 1 {
		t.Fatalf("calls: got %d, want 1", len(plan.Steps[0].Calls))
	}
}

func TestStepLevelLoopReplacesParent(t *testing.T) {
	p := &pipe.Pipe{
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{"n": "1..3"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// Step loop replaces parent: 3 calls, not 2
	calls := plan.Steps[0].Calls
	if len(calls) != 3 {
		t.Fatalf("calls: got %d, want 3", len(calls))
	}
	if _, ok := calls[0].Params["date"]; ok {
		t.Error("should not have parent loop key 'date'")
	}
	if calls[0].Params["n"] != "1" {
		t.Errorf("n = %q, want 1", calls[0].Params["n"])
	}
}

func TestNestedPipeWithLoop(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"src": "/data"},
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "pipes/child.pipe.yaml", With: pipe.StringMap{"quality": "90"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// Nested pipe inherits pipe-level loop: 2 dates = 2 calls
	if len(plan.Steps[0].Calls) != 2 {
		t.Fatalf("calls: got %d, want 2", len(plan.Steps[0].Calls))
	}
	for _, call := range plan.Steps[0].Calls {
		if call.Job != "pipes/child.pipe.yaml" {
			t.Errorf("job = %q", call.Job)
		}
		if call.Params["src"] != "/data" {
			t.Errorf("src = %q", call.Params["src"])
		}
		if call.Params["quality"] != "90" {
			t.Errorf("quality = %q", call.Params["quality"])
		}
	}
	if plan.Steps[0].Calls[0].Params["date"] == plan.Steps[0].Calls[1].Params["date"] {
		t.Error("expected different date values for each call")
	}
}

func TestNestedPipeWithEach(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{
				Job: "pipes/child.pipe.yaml",
				Each: []pipe.StringMap{
					{"project": "ds"},
					{"project": "hn2"},
				},
			},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps[0].Calls) != 2 {
		t.Fatalf("calls: got %d, want 2", len(plan.Steps[0].Calls))
	}
	if plan.Steps[0].Calls[0].Params["project"] != "ds" {
		t.Errorf("call 0 project = %q, want ds", plan.Steps[0].Calls[0].Params["project"])
	}
	if plan.Steps[0].Calls[1].Params["project"] != "hn2" {
		t.Errorf("call 1 project = %q, want hn2", plan.Steps[0].Calls[1].Params["project"])
	}
}

func TestNestedPipeNoLoop(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"src": "/data"},
		Steps: []pipe.Step{
			{Job: "pipes/child.pipe.yaml", With: pipe.StringMap{"quality": "90"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps[0].Calls) != 1 {
		t.Fatalf("calls: got %d, want 1", len(plan.Steps[0].Calls))
	}
	if plan.Steps[0].Calls[0].Params["src"] != "/data" {
		t.Errorf("src = %q", plan.Steps[0].Calls[0].Params["src"])
	}
}

func TestTemplateSubstitution(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"base": "/data/output"},
		Each: []pipe.StringMap{
			{"label": "fullhd"},
		},
		Loop: map[string]any{"date": "-1d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py", With: pipe.StringMap{
				"output": "{base}/{label}/{date}.jpg",
			}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	call := plan.Steps[0].Calls[0]
	if call.Params["output"] != "/data/output/fullhd/2026-03-18.jpg" {
		t.Errorf("output = %q", call.Params["output"])
	}
}

func TestTemplateSameKeyFromPipeWith(t *testing.T) {
	// Step with: output_dir: "{output_dir}" should resolve from pipe-level with
	p := &pipe.Pipe{
		With: pipe.StringMap{"output_dir": "/tmp/demo"},
		Steps: []pipe.Step{
			{Job: "scripts/run.py", With: pipe.StringMap{
				"output_dir":  "{output_dir}",
				"report_file": "{output_dir}/report.txt",
			}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	call := plan.Steps[0].Calls[0]
	if call.Params["output_dir"] != "/tmp/demo" {
		t.Errorf("output_dir = %q, want /tmp/demo", call.Params["output_dir"])
	}
	if call.Params["report_file"] != "/tmp/demo/report.txt" {
		t.Errorf("report_file = %q, want /tmp/demo/report.txt", call.Params["report_file"])
	}
}

func TestTemplateForwardReference(t *testing.T) {
	// pipe with references a value that comes from CLI overrides (later layer)
	p := &pipe.Pipe{
		With: pipe.StringMap{
			"table": "{project}_events",
		},
		Steps: []pipe.Step{
			{Job: "scripts/dau.py", With: pipe.StringMap{
				"target": "{project}.mart_dau",
			}},
		},
	}
	overrides := map[string]string{"project": "mygame"}
	plan, err := Expand(p, overrides, now)
	if err != nil {
		t.Fatal(err)
	}
	call := plan.Steps[0].Calls[0]
	if call.Params["table"] != "mygame_events" {
		t.Errorf("table = %q, want %q", call.Params["table"], "mygame_events")
	}
	if call.Params["target"] != "mygame.mart_dau" {
		t.Errorf("target = %q, want %q", call.Params["target"], "mygame.mart_dau")
	}
	if call.Params["project"] != "mygame" {
		t.Errorf("project = %q, want %q", call.Params["project"], "mygame")
	}
}

func TestOverrideWins(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"quality": "80"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py", With: pipe.StringMap{"quality": "100"}},
		},
	}
	overrides := map[string]string{"quality": "110"}
	plan, err := Expand(p, overrides, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Params["quality"] != "110" {
		t.Errorf("quality = %q, want 110", plan.Steps[0].Calls[0].Params["quality"])
	}
}

func TestTimeExprInWith(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"date": "-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Params["date"] != "2026-03-18" {
		t.Errorf("date = %q", plan.Steps[0].Calls[0].Params["date"])
	}
}

func TestTimeExprInEach(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"date": "-1d"},
		},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Params["date"] != "2026-03-18" {
		t.Errorf("date = %q", plan.Steps[0].Calls[0].Params["date"])
	}
}

func TestNonTimeExprUnchanged(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"size": "1920x1080"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Params["size"] != "1920x1080" {
		t.Errorf("size = %q", plan.Steps[0].Calls[0].Params["size"])
	}
}

func TestInputModeDefault(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Input != pipe.InputEnv {
		t.Errorf("input = %q, want env", plan.Steps[0].Calls[0].Input)
	}
}

func TestInputModePipeLevel(t *testing.T) {
	p := &pipe.Pipe{
		Input: pipe.InputJSON,
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Input != pipe.InputJSON {
		t.Errorf("input = %q, want json", plan.Steps[0].Calls[0].Input)
	}
}

func TestInputModeStepOverrides(t *testing.T) {
	p := &pipe.Pipe{
		Input: pipe.InputJSON,
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Input: pipe.InputArgs},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Calls[0].Input != pipe.InputArgs {
		t.Errorf("input = %q, want args", plan.Steps[0].Calls[0].Input)
	}
}

func TestRetryInherit(t *testing.T) {
	p := &pipe.Pipe{
		Retry: intPtr(3),
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Retry != 3 {
		t.Errorf("retry = %d, want 3", plan.Steps[0].Retry)
	}
}

func TestRetryStepOverride(t *testing.T) {
	p := &pipe.Pipe{
		Retry: intPtr(3),
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Retry: intPtr(5)},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Retry != 5 {
		t.Errorf("retry = %d, want 5", plan.Steps[0].Retry)
	}
}

func TestRetryOff(t *testing.T) {
	p := &pipe.Pipe{
		Retry: intPtr(3),
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", RetryOff: true},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Retry != 0 {
		t.Errorf("retry = %d, want 0", plan.Steps[0].Retry)
	}
}

func TestRetryDelayDefault(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].RetryDelay != time.Second {
		t.Errorf("retry_delay = %v, want 1s", plan.Steps[0].RetryDelay)
	}
}

func TestRetryDelayOverride(t *testing.T) {
	p := &pipe.Pipe{
		RetryDelay: "5s",
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", RetryDelay: "10s"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].RetryDelay != 10*time.Second {
		t.Errorf("retry_delay = %v, want 10s", plan.Steps[0].RetryDelay)
	}
}

func TestTimeout(t *testing.T) {
	p := &pipe.Pipe{
		Timeout: "10m",
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Timeout: "30m"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].Timeout != 30*time.Minute {
		t.Errorf("timeout = %v, want 30m", plan.Steps[0].Timeout)
	}
}

func TestAllowFailure(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/notify.sh", AllowFailure: true},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Steps[0].AllowFailure {
		t.Error("allow_failure = false, want true")
	}
}

func TestDimsString(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"size": "1920x1080"},
			{"size": "1280x720"},
			{"size": "640x480"},
			{"size": "128x128"},
		},
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/resize.py"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	dims := plan.Steps[0].Dims
	if dims != "4 each × 2 date" {
		t.Errorf("dims = %q, want %q", dims, "4 each × 2 date")
	}
}

func TestTotalCalls(t *testing.T) {
	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"size": "1920x1080"},
			{"size": "128x128"},
		},
		Loop: map[string]any{"date": "-2d..-1d"},
		Steps: []pipe.Step{
			{Job: "scripts/download.sh", EachOff: true},         // 2
			{Job: "scripts/resize.py"},                           // 4
			{Job: "scripts/upload.sh", EachOff: true, LoopOff: true}, // 1
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalCalls() != 7 {
		t.Errorf("total = %d, want 7", plan.TotalCalls())
	}
}

func TestDescriptionAndLog(t *testing.T) {
	p := &pipe.Pipe{
		Description: "Test pipe",
		Log:         []string{"label", "file"},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Description != "Test pipe" {
		t.Errorf("description = %q", plan.Description)
	}
	if len(plan.Log) != 2 || plan.Log[0] != "label" {
		t.Errorf("log = %v", plan.Log)
	}
}

// --- Environment variable interpolation ---

func TestEnvVarInPipeWith(t *testing.T) {
	os.Setenv("PIPERIG_TEST_HOST", "db.example.com")
	defer os.Unsetenv("PIPERIG_TEST_HOST")

	p := &pipe.Pipe{
		With: pipe.StringMap{"host": "$PIPERIG_TEST_HOST"},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["host"]
	if got != "db.example.com" {
		t.Errorf("host = %q, want %q", got, "db.example.com")
	}
}

func TestEnvVarBracesSyntax(t *testing.T) {
	os.Setenv("PIPERIG_TEST_BUCKET", "my-bucket")
	defer os.Unsetenv("PIPERIG_TEST_BUCKET")

	p := &pipe.Pipe{
		With: pipe.StringMap{"path": "s3://${PIPERIG_TEST_BUCKET}/output"},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["path"]
	if got != "s3://my-bucket/output" {
		t.Errorf("path = %q, want %q", got, "s3://my-bucket/output")
	}
}

func TestEnvVarUnset(t *testing.T) {
	os.Unsetenv("PIPERIG_TEST_MISSING")

	p := &pipe.Pipe{
		With: pipe.StringMap{"val": "$PIPERIG_TEST_MISSING"},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["val"]
	if got != "" {
		t.Errorf("val = %q, want empty string", got)
	}
}

func TestEnvVarInStepWith(t *testing.T) {
	os.Setenv("PIPERIG_TEST_KEY", "secret123")
	defer os.Unsetenv("PIPERIG_TEST_KEY")

	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", With: pipe.StringMap{"api_key": "$PIPERIG_TEST_KEY"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["api_key"]
	if got != "secret123" {
		t.Errorf("api_key = %q, want %q", got, "secret123")
	}
}

func TestEnvVarInEach(t *testing.T) {
	os.Setenv("PIPERIG_TEST_REGION", "eu-west-1")
	defer os.Unsetenv("PIPERIG_TEST_REGION")

	p := &pipe.Pipe{
		Each: []pipe.StringMap{
			{"region": "$PIPERIG_TEST_REGION"},
		},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["region"]
	if got != "eu-west-1" {
		t.Errorf("region = %q, want %q", got, "eu-west-1")
	}
}

func TestEnvVarNoExpansionWithoutDollar(t *testing.T) {
	p := &pipe.Pipe{
		With: pipe.StringMap{"plain": "no-env-here"},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["plain"]
	if got != "no-env-here" {
		t.Errorf("plain = %q, want %q", got, "no-env-here")
	}
}

func TestEnvVarInLoopString(t *testing.T) {
	os.Setenv("PIPERIG_TEST_RANGE", "-2d..-1d")
	defer os.Unsetenv("PIPERIG_TEST_RANGE")

	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{"date": "$PIPERIG_TEST_RANGE"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps[0].Calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(plan.Steps[0].Calls))
	}
}

func TestEnvVarInLoopList(t *testing.T) {
	os.Setenv("PIPERIG_TEST_EXTRA", "asia")
	defer os.Unsetenv("PIPERIG_TEST_EXTRA")

	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{
				"region": []any{"eu", "$PIPERIG_TEST_EXTRA"},
			}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	calls := plan.Steps[0].Calls
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].Params["region"] != "eu" {
		t.Errorf("call[0] region = %q, want %q", calls[0].Params["region"], "eu")
	}
	if calls[1].Params["region"] != "asia" {
		t.Errorf("call[1] region = %q, want %q", calls[1].Params["region"], "asia")
	}
}

func TestEnvVarPlusTemplate(t *testing.T) {
	os.Setenv("PIPERIG_TEST_BASE", "/data/output")
	defer os.Unsetenv("PIPERIG_TEST_BASE")

	p := &pipe.Pipe{
		With: pipe.StringMap{"base": "$PIPERIG_TEST_BASE"},
		Each: []pipe.StringMap{
			{"label": "fullhd"},
		},
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", With: pipe.StringMap{"output": "{base}/{label}.jpg"}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["output"]
	if got != "/data/output/fullhd.jpg" {
		t.Errorf("output = %q, want %q", got, "/data/output/fullhd.jpg")
	}
}

func TestEnvVarInLoopPlainString(t *testing.T) {
	os.Setenv("PIPERIG_TEST_REGION", "eu-west-1")
	defer os.Unsetenv("PIPERIG_TEST_REGION")

	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{
				"region": "$PIPERIG_TEST_REGION",
			}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps[0].Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(plan.Steps[0].Calls))
	}
	got := plan.Steps[0].Calls[0].Params["region"]
	if got != "eu-west-1" {
		t.Errorf("region = %q, want %q", got, "eu-west-1")
	}
}

func TestEnvVarInLoopUnset(t *testing.T) {
	os.Unsetenv("PIPERIG_TEST_GONE")

	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", Loop: map[string]any{
				"x": "$PIPERIG_TEST_GONE",
			}},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Steps[0].Calls[0].Params["x"]
	if got != "" {
		t.Errorf("x = %q, want empty string", got)
	}
}

func TestHookInheritFromPipe(t *testing.T) {
	p := &pipe.Pipe{
		OnFail:    "scripts/alert.sh",
		OnSuccess: "scripts/notify.sh",
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].OnFail != "scripts/alert.sh" {
		t.Errorf("OnFail = %q, want %q", plan.Steps[0].OnFail, "scripts/alert.sh")
	}
	if plan.Steps[0].OnSuccess != "scripts/notify.sh" {
		t.Errorf("OnSuccess = %q, want %q", plan.Steps[0].OnSuccess, "scripts/notify.sh")
	}
}

func TestHookStepOverride(t *testing.T) {
	p := &pipe.Pipe{
		OnFail: "scripts/alert.sh",
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", OnFail: "scripts/custom-alert.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].OnFail != "scripts/custom-alert.sh" {
		t.Errorf("OnFail = %q, want %q", plan.Steps[0].OnFail, "scripts/custom-alert.sh")
	}
}

func TestHookOff(t *testing.T) {
	p := &pipe.Pipe{
		OnFail:    "scripts/alert.sh",
		OnSuccess: "scripts/notify.sh",
		Steps: []pipe.Step{
			{Job: "scripts/run.sh", OnFailOff: true, OnSuccessOff: true},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].OnFail != "" {
		t.Errorf("OnFail = %q, want empty", plan.Steps[0].OnFail)
	}
	if plan.Steps[0].OnSuccess != "" {
		t.Errorf("OnSuccess = %q, want empty", plan.Steps[0].OnSuccess)
	}
}

func TestHookNoDefault(t *testing.T) {
	p := &pipe.Pipe{
		Steps: []pipe.Step{
			{Job: "scripts/run.sh"},
		},
	}
	plan, err := Expand(p, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].OnFail != "" {
		t.Errorf("OnFail = %q, want empty", plan.Steps[0].OnFail)
	}
	if plan.Steps[0].OnSuccess != "" {
		t.Errorf("OnSuccess = %q, want empty", plan.Steps[0].OnSuccess)
	}
}
