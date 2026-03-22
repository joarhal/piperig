package expand

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joarhal/piperig/internal/pipe"
	"github.com/joarhal/piperig/internal/timeexpr"
)

const defaultRetryDelay = time.Second

// Expand transforms a Pipe into a Plan with fully resolved Calls.
// Pure logic — no I/O, deterministic.
func Expand(p *pipe.Pipe, overrides map[string]string, now time.Time) (*pipe.Plan, error) {
	// Step 1: resolve time expressions in pipe-level params
	pipeWith := cloneMap(map[string]string(p.With))
	if err := resolveTimeExprs(pipeWith, now); err != nil {
		return nil, fmt.Errorf("pipe with: %w", err)
	}

	pipeEach, err := resolveEachTimeExprs(p.Each, now)
	if err != nil {
		return nil, fmt.Errorf("pipe each: %w", err)
	}

	pipeLoopKeys, pipeLoopVals, err := expandLoopValues(p.Loop, now)
	if err != nil {
		return nil, fmt.Errorf("pipe loop: %w", err)
	}

	plan := &pipe.Plan{
		Description: p.Description,
		Log:         p.Log,
	}

	for i, step := range p.Steps {
		sp, err := expandStep(step, p, pipeWith, pipeEach, pipeLoopKeys, pipeLoopVals, overrides, now)
		if err != nil {
			return nil, fmt.Errorf("step %d (%s): %w", i+1, step.Job, err)
		}
		plan.Steps = append(plan.Steps, *sp)
	}

	return plan, nil
}

func expandStep(
	step pipe.Step,
	p *pipe.Pipe,
	pipeWith map[string]string,
	pipeEach []map[string]string,
	pipeLoopKeys []string,
	pipeLoopVals [][]string,
	overrides map[string]string,
	now time.Time,
) (*pipe.StepPlan, error) {
	// Resolve time expressions in step with
	stepWith := cloneMap(map[string]string(step.With))
	if err := resolveTimeExprs(stepWith, now); err != nil {
		return nil, fmt.Errorf("with: %w", err)
	}

	// Nested pipe: 1 call, no expansion
	if strings.HasSuffix(step.Job, ".pipe.yaml") {
		params := mergeParams(pipeWith, stepWith, overrides)
		call := pipe.Call{
			Job:    step.Job,
			Params: params,
			Input:  resolveInput(step, p),
		}
		sp := &pipe.StepPlan{
			Job:   step.Job,
			Calls: []pipe.Call{call},
			Log:   resolveLog(step, p),
		}
		applyPolicies(sp, step, p)
		return sp, nil
	}

	// Determine effective each
	var effEach []map[string]string
	if step.EachOff {
		effEach = nil
	} else if step.Each != nil {
		effEach = toStringMaps(step.Each)
		var err error
		effEach, err = resolveEachTimeExprsRaw(effEach, now)
		if err != nil {
			return nil, fmt.Errorf("each: %w", err)
		}
	} else {
		effEach = pipeEach
	}

	// Determine effective loop
	var loopKeys []string
	var loopVals [][]string
	if step.LoopOff {
		loopKeys = nil
		loopVals = nil
	} else if step.Loop != nil {
		var err error
		loopKeys, loopVals, err = expandLoopValues(step.Loop, now)
		if err != nil {
			return nil, fmt.Errorf("loop: %w", err)
		}
	} else {
		loopKeys = pipeLoopKeys
		loopVals = pipeLoopVals
	}

	// Build dims string
	dims := buildDims(len(effEach), loopKeys, loopVals)

	// Generate cartesian product of loop values
	loopCombinations := cartesian(loopKeys, loopVals)
	if len(loopCombinations) == 0 {
		loopCombinations = []map[string]string{nil}
	}

	// Each defaults to single empty item if nil
	if len(effEach) == 0 {
		effEach = []map[string]string{nil}
	}

	// Build calls: each × loop
	var calls []pipe.Call
	for _, eachItem := range effEach {
		for _, loopItem := range loopCombinations {
			params := mergeParams(pipeWith, eachItem, loopItem, stepWith, overrides)
			calls = append(calls, pipe.Call{
				Job:    step.Job,
				Params: params,
				Input:  resolveInput(step, p),
			})
		}
	}

	sp := &pipe.StepPlan{
		Job:   step.Job,
		Calls: calls,
		Dims:  dims,
		Log:   resolveLog(step, p),
	}
	applyPolicies(sp, step, p)
	return sp, nil
}

// resolveTimeExprs resolves time expressions in-place in a param map.
func resolveTimeExprs(params map[string]string, now time.Time) error {
	for k, v := range params {
		if timeexpr.IsTimeExpr(v) {
			resolved, err := timeexpr.Resolve(v, now)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
			params[k] = resolved
		}
	}
	return nil
}

func resolveEachTimeExprs(each []pipe.StringMap, now time.Time) ([]map[string]string, error) {
	result := toStringMaps(each)
	return resolveEachTimeExprsRaw(result, now)
}

func resolveEachTimeExprsRaw(each []map[string]string, now time.Time) ([]map[string]string, error) {
	for _, item := range each {
		if err := resolveTimeExprs(item, now); err != nil {
			return nil, err
		}
	}
	return each, nil
}

// expandLoopValues expands loop map values into parallel key/value lists.
func expandLoopValues(loop map[string]any, now time.Time) ([]string, [][]string, error) {
	if len(loop) == 0 {
		return nil, nil, nil
	}

	// Sort keys for deterministic order
	keys := make([]string, 0, len(loop))
	for k := range loop {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	values := make([][]string, len(keys))
	for i, k := range keys {
		v := loop[k]
		vals, err := expandSingleLoopValue(v, now)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", k, err)
		}
		values[i] = vals
	}
	return keys, values, nil
}

func expandSingleLoopValue(v any, now time.Time) ([]string, error) {
	switch val := v.(type) {
	case string:
		// Time expression range
		if timeexpr.IsRange(val) {
			return timeexpr.ExpandRange(val, now)
		}
		// Numeric range
		if parts := strings.SplitN(val, "..", 2); len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				return numericRange(start, end)
			}
		}
		// Single time expression
		if timeexpr.IsTimeExpr(val) {
			resolved, err := timeexpr.Resolve(val, now)
			if err != nil {
				return nil, err
			}
			return []string{resolved}, nil
		}
		// Plain string
		return []string{val}, nil

	case []any:
		result := make([]string, len(val))
		for i, item := range val {
			s := fmt.Sprint(item)
			if timeexpr.IsTimeExpr(s) {
				resolved, err := timeexpr.Resolve(s, now)
				if err != nil {
					return nil, err
				}
				result[i] = resolved
			} else {
				result[i] = s
			}
		}
		return result, nil

	case int:
		return []string{strconv.Itoa(val)}, nil

	default:
		return []string{fmt.Sprint(val)}, nil
	}
}

func numericRange(start, end int) ([]string, error) {
	if start > end {
		return nil, fmt.Errorf("inverted numeric range: %d > %d", start, end)
	}
	result := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		result = append(result, strconv.Itoa(i))
	}
	return result, nil
}

// cartesian generates the cartesian product of loop key/value lists.
// Each result is a map of key→value for one combination.
func cartesian(keys []string, values [][]string) []map[string]string {
	if len(keys) == 0 {
		return nil
	}

	// Start with first key's values
	result := make([]map[string]string, 0)
	for _, v := range values[0] {
		result = append(result, map[string]string{keys[0]: v})
	}

	// Multiply with remaining keys
	for i := 1; i < len(keys); i++ {
		var next []map[string]string
		for _, existing := range result {
			for _, v := range values[i] {
				combo := cloneMap(existing)
				combo[keys[i]] = v
				next = append(next, combo)
			}
		}
		result = next
	}

	return result
}

// mergeParams merges parameter maps in priority order (later wins).
// Template substitution happens during merge: each layer's values are
// substituted using the pool built from all previous layers. This means
// step-level `with: output: "{base_dir}/out"` correctly resolves {base_dir}
// from pipe-level with, even if the step doesn't define base_dir itself.
func mergeParams(sources ...map[string]string) map[string]string {
	pool := make(map[string]string)
	for _, src := range sources {
		// Substitute templates in this layer using the pool so far
		for k, v := range src {
			pool[k] = substituteValue(v, pool)
		}
	}
	return pool
}

func substituteValue(val string, params map[string]string) string {
	if !strings.Contains(val, "{") {
		return val
	}
	for k, v := range params {
		val = strings.ReplaceAll(val, "{"+k+"}", v)
	}
	return val
}

func resolveLog(step pipe.Step, p *pipe.Pipe) []string {
	if step.Log != nil {
		return step.Log
	}
	return p.Log
}

func resolveInput(step pipe.Step, p *pipe.Pipe) pipe.InputMode {
	if step.Input != "" {
		return step.Input
	}
	if p.Input != "" {
		return p.Input
	}
	return pipe.InputEnv
}

func applyPolicies(sp *pipe.StepPlan, step pipe.Step, p *pipe.Pipe) {
	// Retry
	switch {
	case step.RetryOff:
		sp.Retry = 0
	case step.Retry != nil:
		sp.Retry = *step.Retry
	case p.Retry != nil:
		sp.Retry = *p.Retry
	}

	// RetryDelay
	sp.RetryDelay = defaultRetryDelay
	if p.RetryDelay != "" {
		if d, err := time.ParseDuration(p.RetryDelay); err == nil {
			sp.RetryDelay = d
		}
	}
	if step.RetryDelay != "" {
		if d, err := time.ParseDuration(step.RetryDelay); err == nil {
			sp.RetryDelay = d
		}
	}

	// Timeout
	if p.Timeout != "" {
		if d, err := time.ParseDuration(p.Timeout); err == nil {
			sp.Timeout = d
		}
	}
	if step.Timeout != "" {
		if d, err := time.ParseDuration(step.Timeout); err == nil {
			sp.Timeout = d
		}
	}

	// AllowFailure
	sp.AllowFailure = step.AllowFailure
}

func buildDims(eachLen int, loopKeys []string, loopVals [][]string) string {
	var parts []string
	if eachLen > 0 {
		parts = append(parts, fmt.Sprintf("%d each", eachLen))
	}
	for i, k := range loopKeys {
		parts = append(parts, fmt.Sprintf("%d %s", len(loopVals[i]), k))
	}
	return strings.Join(parts, " × ")
}

func cloneMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func toStringMaps(sms []pipe.StringMap) []map[string]string {
	if sms == nil {
		return nil
	}
	result := make([]map[string]string, len(sms))
	for i, sm := range sms {
		result[i] = cloneMap(map[string]string(sm))
	}
	return result
}
