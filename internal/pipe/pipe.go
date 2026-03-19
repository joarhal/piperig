package pipe

import "time"

// InputMode determines how parameters are passed to a job.
type InputMode string

const (
	InputEnv  InputMode = "env"  // environment variables (default)
	InputJSON InputMode = "json" // JSON on stdin
	InputArgs InputMode = "args" // CLI arguments
)

// StringMap is a map[string]string that normalizes YAML scalar values
// (int, bool, float) to strings during unmarshalling.
type StringMap map[string]string

// Pipe is the top-level structure parsed from a .pipe.yaml file.
type Pipe struct {
	Description string    `yaml:"description"`
	With        StringMap `yaml:"with"`
	Loop        map[string]any `yaml:"loop"`
	Each        []StringMap    `yaml:"each"`
	Input       InputMode `yaml:"input"`
	Log         []string  `yaml:"log"`
	Retry       *int      `yaml:"retry"`
	RetryDelay  string    `yaml:"retry_delay"`
	Timeout     string    `yaml:"timeout"`
	Steps       []Step    `yaml:"steps"`
}

// Step is a single step within a pipe.
// Custom UnmarshalYAML handles polymorphic fields (loop, each, retry)
// that can be either their normal type or the boolean false.
type Step struct {
	Job          string         `yaml:"job"`
	With         StringMap      `yaml:"with"`
	Loop         map[string]any `yaml:"-"`
	LoopOff      bool           `yaml:"-"`
	Each         []StringMap    `yaml:"-"`
	EachOff      bool           `yaml:"-"`
	Input        InputMode      `yaml:"input"`
	Retry        *int           `yaml:"-"`
	RetryOff     bool           `yaml:"-"`
	RetryDelay   string         `yaml:"retry_delay"`
	Timeout      string         `yaml:"timeout"`
	AllowFailure bool           `yaml:"allow_failure"`
}

// Call is the central type produced by expansion and consumed by runner.
type Call struct {
	Job    string
	Params map[string]string
	Input  InputMode
}

// StepPlan is the result of expanding a single step.
type StepPlan struct {
	Job          string
	Calls        []Call
	Dims         string // e.g. "4 each × 2 dates" — for check output
	Retry        int
	RetryDelay   time.Duration
	Timeout      time.Duration
	AllowFailure bool
}

// Plan is the result of expanding an entire pipe.
type Plan struct {
	Description string
	Log         []string
	Steps       []StepPlan
}

// TotalCalls returns the total number of calls across all steps.
func (p *Plan) TotalCalls() int {
	n := 0
	for _, s := range p.Steps {
		n += len(s.Calls)
	}
	return n
}
