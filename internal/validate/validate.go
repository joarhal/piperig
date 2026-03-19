package validate

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

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

// Rule 4+5: nested .pipe.yaml files exist; no loop/each on nested steps.
func checkNestedPipes(p *pipe.Pipe, fileExists func(string) bool) []error {
	var errs []error
	for i, step := range p.Steps {
		if !strings.HasSuffix(step.Job, ".pipe.yaml") {
			continue
		}
		if !fileExists(step.Job) {
			errs = append(errs, fmt.Errorf("step %d: nested pipe not found: %s", i+1, step.Job))
		}
		if step.Loop != nil && !step.LoopOff {
			errs = append(errs, fmt.Errorf("step %d: loop not allowed on nested pipe", i+1))
		}
		if step.Each != nil && !step.EachOff {
			errs = append(errs, fmt.Errorf("step %d: each not allowed on nested pipe", i+1))
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
		return fmt.Errorf("%s: invalid input mode %q", context, mode)
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

func checkTemplateValue(val string, keys map[string]bool, context string) []error {
	var errs []error
	matches := templateRe.FindAllStringSubmatch(val, -1)
	for _, m := range matches {
		key := m[1]
		if !keys[key] {
			errs = append(errs, fmt.Errorf("%s: template {%s} unresolved", context, key))
		}
	}
	return errs
}
