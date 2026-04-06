package validate

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joarhal/piperig/internal/config"
	"github.com/joarhal/piperig/internal/pipe"
	"github.com/joarhal/piperig/internal/timeexpr"
)

var templateRe = regexp.MustCompile(`\{([^}]+)\}`)

// Validate checks a Pipe against all validation rules.
// Returns a slice of errors (empty = valid).
// fileExists is injected for testability.
func Validate(p *pipe.Pipe, cfg *config.Config, fileExists func(string) bool, overrides map[string]string) []error {
	var errs []error

	errs = append(errs, checkJobFiles(p, cfg, fileExists)...)
	errs = append(errs, checkNestedPipes(p, fileExists)...)
	errs = append(errs, checkInputModes(p)...)
	errs = append(errs, checkTimeExprs(p)...)
	errs = append(errs, checkTemplates(p, overrides)...)
	errs = append(errs, checkDurations(p)...)
	errs = append(errs, checkHooks(p, cfg, fileExists)...)

	return errs
}

// Rule 2+3: job files exist and have supported extensions.
func checkJobFiles(p *pipe.Pipe, cfg *config.Config, fileExists func(string) bool) []error {
	var errs []error
	for i, step := range p.Steps {
		if strings.HasSuffix(step.Job, ".pipe.yaml") {
			continue // handled by checkNestedPipes
		}
		if !fileExists(step.Job) {
			errs = append(errs, fmt.Errorf("step %d: file not found: %s", i+1, step.Job))
		}
		ext := filepath.Ext(step.Job)
		if ext == "" {
			continue // no extension = direct exec
		}
		if _, ok := cfg.Interpreters[ext]; !ok {
			errs = append(errs, fmt.Errorf("step %d: unsupported extension %q", i+1, ext))
		}
	}
	return errs
}

// Rule 4: nested .pipe.yaml files exist.
func checkNestedPipes(p *pipe.Pipe, fileExists func(string) bool) []error {
	var errs []error
	for i, step := range p.Steps {
		if !strings.HasSuffix(step.Job, ".pipe.yaml") {
			continue
		}
		if !fileExists(step.Job) {
			errs = append(errs, fmt.Errorf("step %d: nested pipe not found: %s", i+1, step.Job))
		}
	}
	return errs
}

// Rule 6: input mode must be env, json, args, or empty.
func checkInputModes(p *pipe.Pipe) []error {
	var errs []error
	if err := validateInputMode(p.Input, "pipe"); err != nil {
		errs = append(errs, err)
	}
	for i, step := range p.Steps {
		if err := validateInputMode(step.Input, fmt.Sprintf("step %d", i+1)); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateInputMode(mode pipe.InputMode, context string) error {
	switch mode {
	case "", pipe.InputEnv, pipe.InputJSON, pipe.InputArgs:
		return nil
	default:
		return fmt.Errorf("%s: invalid input mode %q (valid: env, json, args)", context, mode)
	}
}

// Rule 7: time expressions in with/each/loop must be valid.
func checkTimeExprs(p *pipe.Pipe) []error {
	var errs []error
	errs = append(errs, checkTimeExprsInMap(p.With, "pipe with")...)
	for i, item := range p.Each {
		errs = append(errs, checkTimeExprsInMap(item, fmt.Sprintf("pipe each[%d]", i))...)
	}
	errs = append(errs, checkTimeExprsInLoop(p.Loop, "pipe loop")...)

	for si, step := range p.Steps {
		prefix := fmt.Sprintf("step %d", si+1)
		errs = append(errs, checkTimeExprsInMap(step.With, prefix+" with")...)
		for i, item := range step.Each {
			errs = append(errs, checkTimeExprsInMap(item, fmt.Sprintf("%s each[%d]", prefix, i))...)
		}
		errs = append(errs, checkTimeExprsInLoop(step.Loop, prefix+" loop")...)
	}
	return errs
}

func checkTimeExprsInMap(m map[string]string, context string) []error {
	var errs []error
	for k, v := range m {
		if looksLikeTimeExpr(v) && !timeexpr.IsTimeExpr(v) {
			errs = append(errs, fmt.Errorf("%s: cannot parse time expression %q (key %s)", context, v, k))
		}
	}
	return errs
}

func checkTimeExprsInLoop(loop map[string]any, context string) []error {
	var errs []error
	for k, v := range loop {
		switch val := v.(type) {
		case string:
			if timeexpr.IsRange(val) {
				continue // valid range
			}
			if looksLikeTimeExpr(val) && !timeexpr.IsTimeExpr(val) {
				errs = append(errs, fmt.Errorf("%s: cannot parse time expression %q (key %s)", context, val, k))
			}
		case []any:
			for _, item := range val {
				s := fmt.Sprint(item)
				if looksLikeTimeExpr(s) && !timeexpr.IsTimeExpr(s) {
					errs = append(errs, fmt.Errorf("%s: cannot parse time expression %q (key %s)", context, s, k))
				}
			}
		}
	}
	return errs
}

var looksLikeTimeExprRe = regexp.MustCompile(`^[+-]?\d+[a-zA-Z]$`)

// looksLikeTimeExpr checks if a string matches the pattern of a time expression
// (optional sign, digits, single letter). Catches valid exprs (-1d) and
// invalid ones (-1x) alike. Does not match "1920x1080" (too many chars after digits)
// or "hello" (no leading digit/sign).
func looksLikeTimeExpr(s string) bool {
	return looksLikeTimeExprRe.MatchString(s)
}

// Rule 8: templates {key} must resolve from available params.
func checkTemplates(p *pipe.Pipe, overrides map[string]string) []error {
	// Collect all available keys across all sources
	availableKeys := make(map[string]bool)
	for k := range p.With {
		availableKeys[k] = true
	}
	for _, item := range p.Each {
		for k := range item {
			availableKeys[k] = true
		}
	}
	for k := range p.Loop {
		availableKeys[k] = true
	}
	for k := range overrides {
		availableKeys[k] = true
	}

	var errs []error

	for si, step := range p.Steps {
		// Step-level keys
		stepKeys := make(map[string]bool)
		for k := range availableKeys {
			stepKeys[k] = true
		}
		for k := range step.With {
			stepKeys[k] = true
		}
		if step.Loop != nil {
			for k := range step.Loop {
				stepKeys[k] = true
			}
		}
		if step.Each != nil {
			for _, item := range step.Each {
				for k := range item {
					stepKeys[k] = true
				}
			}
		}

		prefix := fmt.Sprintf("step %d", si+1)
		// Check templates in step with values
		for _, v := range step.With {
			errs = append(errs, checkTemplateValue(v, stepKeys, prefix)...)
		}
		// Check templates in pipe with values
		for _, v := range p.With {
			errs = append(errs, checkTemplateValue(v, stepKeys, prefix)...)
		}
	}

	return errs
}

// Rule 9: retry_delay and timeout must be valid Go durations.
func checkDurations(p *pipe.Pipe) []error {
	var errs []error
	if p.RetryDelay != "" {
		if _, err := time.ParseDuration(p.RetryDelay); err != nil {
			errs = append(errs, fmt.Errorf("pipe: invalid retry_delay %q", p.RetryDelay))
		}
	}
	if p.Timeout != "" {
		if _, err := time.ParseDuration(p.Timeout); err != nil {
			errs = append(errs, fmt.Errorf("pipe: invalid timeout %q", p.Timeout))
		}
	}
	for i, step := range p.Steps {
		if step.RetryDelay != "" {
			if _, err := time.ParseDuration(step.RetryDelay); err != nil {
				errs = append(errs, fmt.Errorf("step %d: invalid retry_delay %q", i+1, step.RetryDelay))
			}
		}
		if step.Timeout != "" {
			if _, err := time.ParseDuration(step.Timeout); err != nil {
				errs = append(errs, fmt.Errorf("step %d: invalid timeout %q", i+1, step.Timeout))
			}
		}
	}
	return errs
}

// Rule 12: hook scripts exist and have supported extensions.
func checkHooks(p *pipe.Pipe, cfg *config.Config, fileExists func(string) bool) []error {
	var errs []error
	errs = append(errs, validateHookPath(p.OnFail, "on_fail", "pipe", cfg, fileExists)...)
	errs = append(errs, validateHookPath(p.OnSuccess, "on_success", "pipe", cfg, fileExists)...)
	for i, step := range p.Steps {
		prefix := fmt.Sprintf("step %d", i+1)
		errs = append(errs, validateHookPath(step.OnFail, "on_fail", prefix, cfg, fileExists)...)
		errs = append(errs, validateHookPath(step.OnSuccess, "on_success", prefix, cfg, fileExists)...)
	}
	return errs
}

func validateHookPath(path, hookName, context string, cfg *config.Config, fileExists func(string) bool) []error {
	if path == "" {
		return nil
	}
	var errs []error
	if strings.HasSuffix(path, ".pipe.yaml") {
		errs = append(errs, fmt.Errorf("%s %s: hooks cannot be .pipe.yaml files: %s", context, hookName, path))
		return errs
	}
	if !fileExists(path) {
		errs = append(errs, fmt.Errorf("%s %s: file not found: %s", context, hookName, path))
	}
	ext := filepath.Ext(path)
	if ext != "" {
		if _, ok := cfg.Interpreters[ext]; !ok {
			errs = append(errs, fmt.Errorf("%s %s: unsupported extension %q", context, hookName, ext))
		}
	}
	return errs
}

func checkTemplateValue(val string, keys map[string]bool, context string) []error {
	var errs []error
	indices := templateRe.FindAllStringSubmatchIndex(val, -1)
	for _, idx := range indices {
		// Skip ${VAR} — that's env var interpolation, not a piperig template
		if idx[0] > 0 && val[idx[0]-1] == '$' {
			continue
		}
		key := val[idx[2]:idx[3]]
		if !keys[key] {
			errs = append(errs, fmt.Errorf("%s: template {%s} unresolved", context, key))
		}
	}
	return errs
}
