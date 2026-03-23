package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/joarhal/piperig/internal/pipe"
)

func newTestWriter() (*Writer, *bytes.Buffer) {
	var buf bytes.Buffer
	return New(&buf, false), &buf
}

func newColorWriter() (*Writer, *bytes.Buffer) {
	var buf bytes.Buffer
	return New(&buf, true), &buf
}

func TestStart(t *testing.T) {
	w, buf := newTestWriter()
	w.Start("scripts/resize.py", map[string]string{"date": "2026-03-18", "size": "1920x1080"})
	out := buf.String()
	if !strings.Contains(out, "→ scripts/resize.py  date=2026-03-18  size=1920x1080") {
		t.Errorf("got %q, want timestamp + → scripts/resize.py ...", out)
	}
	// Should have HH:MM:SS timestamp
	if len(out) < 8 || out[2] != ':' || out[5] != ':' {
		t.Errorf("expected HH:MM:SS timestamp prefix, got %q", out)
	}
}

func TestStartNoParams(t *testing.T) {
	w, buf := newTestWriter()
	w.Start("scripts/run.sh", nil)
	out := buf.String()
	if !strings.Contains(out, "→ scripts/run.sh") {
		t.Errorf("got %q, want timestamp + → scripts/run.sh", out)
	}
}

func TestStartBlankLineBetweenSteps(t *testing.T) {
	w, buf := newTestWriter()
	w.Start("job1", nil)
	first := buf.Len()
	w.Start("job2", nil)
	out := buf.String()[first:]
	if !strings.HasPrefix(out, "\n") {
		t.Error("expected blank line between steps")
	}
}

func TestText(t *testing.T) {
	w, buf := newTestWriter()
	w.Text("Resizing image...")
	out := buf.String()
	if !strings.Contains(out, "· Resizing image...") {
		t.Errorf("got %q, want indent + · Resizing image...", out)
	}
}

func TestJSON(t *testing.T) {
	w, buf := newTestWriter()
	w.SetLog([]string{"label", "file", "size"})
	w.JSON(map[string]string{"label": "fullhd", "file": "photo_001.jpg", "size": "1920x1080"})
	out := buf.String()
	if !strings.Contains(out, "▸ fullhd | photo_001.jpg | 1920x1080") {
		t.Errorf("got %q, want indent + ▸ fullhd | ...", out)
	}
}

func TestJSONMissingField(t *testing.T) {
	w, buf := newTestWriter()
	w.SetLog([]string{"label", "missing", "size"})
	w.JSON(map[string]string{"label": "fullhd", "size": "1920x1080"})
	out := buf.String()
	if !strings.Contains(out, "▸ fullhd | 1920x1080") {
		t.Errorf("got %q, want indent + ▸ fullhd | 1920x1080", out)
	}
}

func TestStderr(t *testing.T) {
	w, buf := newTestWriter()
	w.Stderr("Warning: EXIF data missing")
	out := buf.String()
	if !strings.Contains(out, "! Warning: EXIF data missing") {
		t.Errorf("got %q, want indent + ! Warning: EXIF data missing", out)
	}
}

func TestRetry(t *testing.T) {
	w, buf := newTestWriter()
	w.Retry(1, 3, time.Second)
	out := buf.String()
	if !strings.Contains(out, "↻ retry 1/3 (1s)") {
		t.Errorf("got %q, want indent + ↻ retry 1/3 (1s)", out)
	}
}

func TestOk(t *testing.T) {
	w, buf := newTestWriter()
	w.Ok("scripts/resize.py", 800*time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, "✓ scripts/resize.py  0.8s") {
		t.Errorf("got %q, want timestamp + ✓ scripts/resize.py  0.8s", out)
	}
	if len(out) < 8 || out[2] != ':' || out[5] != ':' {
		t.Errorf("expected HH:MM:SS timestamp prefix, got %q", out)
	}
}

func TestFail(t *testing.T) {
	w, buf := newTestWriter()
	w.Fail("scripts/upload.sh", 1, 4100*time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, "✗ scripts/upload.sh  exit=1  4.1s") {
		t.Errorf("got %q, want timestamp + ✗ scripts/upload.sh  exit=1  4.1s", out)
	}
	if len(out) < 8 || out[2] != ':' || out[5] != ':' {
		t.Errorf("expected HH:MM:SS timestamp prefix, got %q", out)
	}
}

func TestCheckPipe(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckPipe("images.pipe.yaml", "Resize images for the last 2 days")
	want := "Pipe: images.pipe.yaml (Resize images for the last 2 days)\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckPipeNoDesc(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckPipe("images.pipe.yaml", "")
	want := "Pipe: images.pipe.yaml\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckStep(t *testing.T) {
	w, buf := newTestWriter()
	step := pipe.StepPlan{
		Job:   "scripts/resize.py",
		Calls: make([]pipe.Call, 8),
		Dims:  "4 each × 2 dates",
	}
	w.CheckStep(2, step)
	want := "  Step 2: scripts/resize.py × 4 each × 2 dates = 8 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckStepNoDims(t *testing.T) {
	w, buf := newTestWriter()
	step := pipe.StepPlan{
		Job:   "scripts/upload.sh",
		Calls: make([]pipe.Call, 1),
	}
	w.CheckStep(4, step)
	want := "  Step 4: scripts/upload.sh = 1 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckCall(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckCall(1, map[string]string{"src": "/data", "quality": "80"})
	want := "    1. quality=80  src=/data\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckCallEmptyParams(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckCall(1, nil)
	want := "    1. \n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckTotal(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckTotal(19)
	want := "  Total: 19 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckTotalZero(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckTotal(0)
	want := "  Total: 0 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// --- Color tests ---

func TestColorOutput(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf, true)
	w.Text("hello")
	out := buf.String()
	if out == "  · hello\n" {
		t.Error("expected ANSI codes in color mode")
	}
	if !bytes.Contains(buf.Bytes(), []byte("hello")) {
		t.Error("output should contain the text")
	}
}

func TestPipeHeaderColor(t *testing.T) {
	w, buf := newColorWriter()
	w.PipeHeader("mypipe", "a description")
	out := buf.String()
	if !strings.Contains(out, bold) {
		t.Error("expected bold ANSI code for pipe name")
	}
	if !strings.Contains(out, dim) {
		t.Error("expected dim ANSI code for description")
	}
	if !strings.Contains(out, "mypipe") {
		t.Error("expected pipe name in output")
	}
	if !strings.Contains(out, "a description") {
		t.Error("expected description in output")
	}
	if !strings.Contains(out, reset) {
		t.Error("expected reset ANSI code")
	}
}

func TestPipeHeaderColorNoDesc(t *testing.T) {
	w, buf := newColorWriter()
	w.PipeHeader("mypipe", "")
	out := buf.String()
	if !strings.Contains(out, bold) {
		t.Error("expected bold ANSI code for pipe name")
	}
	if !strings.Contains(out, "mypipe") {
		t.Error("expected pipe name in output")
	}
	// Should not contain dim (no description)
	if strings.Contains(out, dim+"— ") {
		t.Error("should not contain description formatting without description")
	}
}

func TestPipeHeaderNoColor(t *testing.T) {
	w, buf := newTestWriter()
	w.PipeHeader("mypipe", "a description")
	want := "mypipe — a description\n\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestPipeHeaderNoColorNoDesc(t *testing.T) {
	w, buf := newTestWriter()
	w.PipeHeader("mypipe", "")
	want := "mypipe\n\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestPipeSummaryNoColor(t *testing.T) {
	w, buf := newTestWriter()
	w.PipeSummary(5, 3*time.Second, false)
	out := buf.String()
	if !strings.Contains(out, "✓ 5 calls  3.0s") {
		t.Errorf("got %q, want success summary", out)
	}
}

func TestPipeSummaryNoColorFailed(t *testing.T) {
	w, buf := newTestWriter()
	w.PipeSummary(5, 3*time.Second, true)
	out := buf.String()
	if !strings.Contains(out, "✗ 5 calls  3.0s") {
		t.Errorf("got %q, want failed summary", out)
	}
}

func TestPipeSummaryColor(t *testing.T) {
	w, buf := newColorWriter()
	w.PipeSummary(5, 3*time.Second, false)
	out := buf.String()
	if !strings.Contains(out, green) {
		t.Error("expected green ANSI code for success")
	}
	if !strings.Contains(out, "✓") {
		t.Error("expected checkmark in output")
	}
	if !strings.Contains(out, "5 calls") {
		t.Error("expected call count")
	}
	if !strings.Contains(out, reset) {
		t.Error("expected reset ANSI code")
	}
}

func TestPipeSummaryColorFailed(t *testing.T) {
	w, buf := newColorWriter()
	w.PipeSummary(5, 3*time.Second, true)
	out := buf.String()
	if !strings.Contains(out, red) {
		t.Error("expected red ANSI code for failure")
	}
	if !strings.Contains(out, "✗") {
		t.Error("expected X mark in output")
	}
}

func TestStartColor(t *testing.T) {
	w, buf := newColorWriter()
	w.Start("scripts/run.sh", map[string]string{"x": "1"})
	out := buf.String()
	if !strings.Contains(out, bold) {
		t.Error("expected bold ANSI code")
	}
	if !strings.Contains(out, dim) {
		t.Error("expected dim ANSI code for timestamp/params")
	}
	if !strings.Contains(out, "→") {
		t.Error("expected arrow in output")
	}
	if !strings.Contains(out, "scripts/run.sh") {
		t.Error("expected job name")
	}
	if !strings.Contains(out, "x=1") {
		t.Error("expected params")
	}
}

func TestStartColorNoParams(t *testing.T) {
	w, buf := newColorWriter()
	w.Start("scripts/run.sh", nil)
	out := buf.String()
	if !strings.Contains(out, bold) {
		t.Error("expected bold ANSI code")
	}
	if !strings.Contains(out, "→") {
		t.Error("expected arrow in output")
	}
}

func TestOkColor(t *testing.T) {
	w, buf := newColorWriter()
	w.Ok("scripts/resize.py", 800*time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, green) {
		t.Error("expected green ANSI code for success checkmark")
	}
	if !strings.Contains(out, bold) {
		t.Error("expected bold ANSI code for job name")
	}
	if !strings.Contains(out, dim) {
		t.Error("expected dim ANSI code for duration")
	}
	if !strings.Contains(out, "✓") {
		t.Error("expected checkmark")
	}
	if !strings.Contains(out, "scripts/resize.py") {
		t.Error("expected job name")
	}
	if !strings.Contains(out, "0.8s") {
		t.Error("expected duration")
	}
}

func TestFailColor(t *testing.T) {
	w, buf := newColorWriter()
	w.Fail("scripts/upload.sh", 2, 4100*time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, red) {
		t.Error("expected red ANSI code for failure")
	}
	if !strings.Contains(out, bold) {
		t.Error("expected bold ANSI code for job name")
	}
	if !strings.Contains(out, "✗") {
		t.Error("expected X mark")
	}
	if !strings.Contains(out, "exit=2") {
		t.Error("expected exit code")
	}
	if !strings.Contains(out, "scripts/upload.sh") {
		t.Error("expected job name")
	}
}

func TestTextColor(t *testing.T) {
	w, buf := newColorWriter()
	w.Text("hello world")
	out := buf.String()
	if !strings.Contains(out, dim) {
		t.Error("expected dim ANSI code")
	}
	if !strings.Contains(out, "·") {
		t.Error("expected dot marker")
	}
	if !strings.Contains(out, "hello world") {
		t.Error("expected text content")
	}
	if !strings.Contains(out, reset) {
		t.Error("expected reset ANSI code")
	}
}

func TestJSONColor(t *testing.T) {
	w, buf := newColorWriter()
	w.SetLog([]string{"a", "b"})
	w.JSON(map[string]string{"a": "1", "b": "2"})
	out := buf.String()
	if !strings.Contains(out, cyan) {
		t.Error("expected cyan ANSI code for JSON output")
	}
	if !strings.Contains(out, "▸") {
		t.Error("expected arrow marker")
	}
	if !strings.Contains(out, "1 | 2") {
		t.Error("expected joined values")
	}
	if !strings.Contains(out, reset) {
		t.Error("expected reset ANSI code")
	}
}

func TestStderrColor(t *testing.T) {
	w, buf := newColorWriter()
	w.Stderr("oops")
	out := buf.String()
	if !strings.Contains(out, yellow) {
		t.Error("expected yellow ANSI code for stderr")
	}
	if !strings.Contains(out, "!") {
		t.Error("expected bang marker")
	}
	if !strings.Contains(out, "oops") {
		t.Error("expected stderr text")
	}
	if !strings.Contains(out, reset) {
		t.Error("expected reset ANSI code")
	}
}

func TestRetryColor(t *testing.T) {
	w, buf := newColorWriter()
	w.Retry(2, 5, 3*time.Second)
	out := buf.String()
	if !strings.Contains(out, yellow) {
		t.Error("expected yellow ANSI code for retry")
	}
	if !strings.Contains(out, "↻") {
		t.Error("expected retry marker")
	}
	if !strings.Contains(out, "retry 2/5") {
		t.Error("expected retry count")
	}
	if !strings.Contains(out, "(3s)") {
		t.Error("expected delay")
	}
	if !strings.Contains(out, reset) {
		t.Error("expected reset ANSI code")
	}
}

// --- formatDuration edge cases ---

func TestFormatDurationSubSecond(t *testing.T) {
	got := formatDuration(500 * time.Millisecond)
	if got != "0.5s" {
		t.Errorf("got %q, want %q", got, "0.5s")
	}
}

func TestFormatDurationZero(t *testing.T) {
	got := formatDuration(0)
	if got != "0.0s" {
		t.Errorf("got %q, want %q", got, "0.0s")
	}
}

func TestFormatDurationSeconds(t *testing.T) {
	got := formatDuration(3500 * time.Millisecond)
	if got != "3.5s" {
		t.Errorf("got %q, want %q", got, "3.5s")
	}
}

func TestFormatDurationOneMinute(t *testing.T) {
	got := formatDuration(60 * time.Second)
	if got != "1m0s" {
		t.Errorf("got %q, want %q", got, "1m0s")
	}
}

func TestFormatDurationMinutesAndSeconds(t *testing.T) {
	got := formatDuration(90 * time.Second)
	if got != "1m30s" {
		t.Errorf("got %q, want %q", got, "1m30s")
	}
}

func TestFormatDurationMultipleMinutes(t *testing.T) {
	got := formatDuration(5*time.Minute + 30*time.Second)
	if got != "5m30s" {
		t.Errorf("got %q, want %q", got, "5m30s")
	}
}

func TestFormatDurationMinuteRounding(t *testing.T) {
	// 2m30.7s should round to 2m31s
	got := formatDuration(2*time.Minute + 30*time.Second + 700*time.Millisecond)
	if got != "2m31s" {
		t.Errorf("got %q, want %q", got, "2m31s")
	}
}

// --- SetLog / Log ---

func TestSetLogAndLog(t *testing.T) {
	w, _ := newTestWriter()
	fields := []string{"a", "b", "c"}
	w.SetLog(fields)
	got := w.Log()
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("Log() = %v, want %v", got, fields)
	}
}

func TestLogDefaultNil(t *testing.T) {
	w, _ := newTestWriter()
	if w.Log() != nil {
		t.Errorf("expected nil log by default, got %v", w.Log())
	}
}

// --- formatParams ---

func TestFormatParamsSorted(t *testing.T) {
	got := formatParams(map[string]string{"z": "3", "a": "1", "m": "2"})
	want := "a=1  m=2  z=3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatParamsEmpty(t *testing.T) {
	got := formatParams(map[string]string{})
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestFormatParamsNil(t *testing.T) {
	got := formatParams(nil)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestFormatParamsSingleKey(t *testing.T) {
	got := formatParams(map[string]string{"key": "val"})
	want := "key=val"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- PipeSummary leading blank line ---

func TestPipeSummaryLeadingBlankLine(t *testing.T) {
	w, buf := newTestWriter()
	w.PipeSummary(3, time.Second, false)
	out := buf.String()
	if !strings.HasPrefix(out, "\n") {
		t.Error("expected PipeSummary to start with a blank line")
	}
}

// --- CheckStep with many calls ---

func TestCheckStepManyCalls(t *testing.T) {
	w, buf := newTestWriter()
	step := pipe.StepPlan{
		Job:   "scripts/deploy.sh",
		Calls: make([]pipe.Call, 100),
		Dims:  "10 × 10",
	}
	w.CheckStep(1, step)
	want := "  Step 1: scripts/deploy.sh × 10 × 10 = 100 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// --- CheckCall with multiple sorted params ---

func TestCheckCallSortedParams(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckCall(3, map[string]string{"z": "last", "a": "first", "m": "mid"})
	want := "    3. a=first  m=mid  z=last\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}
