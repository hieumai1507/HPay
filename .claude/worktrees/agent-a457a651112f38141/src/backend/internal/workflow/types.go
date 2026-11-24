// Package workflow implements the configurable workflow engine: definitions
// live in the database as JSONB, steps run on Redis Streams consumers, and
// every state change is guarded (docs plans f4).
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type StepType string

const (
	StepAuto          StepType = "auto"
	StepHumanApproval StepType = "human_approval"
)

// OnFailure policies.
const (
	FailPolicyFail       = "fail"
	FailPolicyCompensate = "compensate"
	FailPolicyManual     = "manual"
)

type StepDef struct {
	Name      string          `json:"name"`
	Type      StepType        `json:"type"`
	Handler   string          `json:"handler,omitempty"`
	Retry     int             `json:"retry"`
	TimeoutMs int             `json:"timeout_ms"`
	OnFailure string          `json:"on_failure"`
	Config    json.RawMessage `json:"config,omitempty"`
}

// ParseSteps validates a definition's steps JSONB.
func ParseSteps(raw []byte) ([]StepDef, error) {
	var steps []StepDef
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil, fmt.Errorf("parse steps: %w", err)
	}
	if len(steps) == 0 {
		return nil, errors.New("definition has no steps")
	}
	names := map[string]bool{}
	for i, s := range steps {
		if s.Name == "" {
			return nil, fmt.Errorf("step %d: empty name", i)
		}
		if names[s.Name] {
			return nil, fmt.Errorf("duplicate step name %q", s.Name)
		}
		names[s.Name] = true
		switch s.Type {
		case StepAuto:
			if s.Handler == "" {
				return nil, fmt.Errorf("step %q: auto step needs a handler", s.Name)
			}
		case StepHumanApproval:
			// no handler: engine pauses; approval module resumes (F6)
		default:
			return nil, fmt.Errorf("step %q: unknown type %q", s.Name, s.Type)
		}
		switch s.OnFailure {
		case FailPolicyFail, FailPolicyCompensate, FailPolicyManual, "":
		default:
			return nil, fmt.Errorf("step %q: unknown on_failure %q", s.Name, s.OnFailure)
		}
	}
	if steps[0].Type != StepAuto {
		// Engine.Start dispatches the first step straight to a handler stream,
		// so a definition must open with an auto step (F6 invariant).
		return nil, fmt.Errorf("step %q: first step must be type auto", steps[0].Name)
	}
	return steps, nil
}

// StepIndex returns the position of a step by name, or -1.
func StepIndex(steps []StepDef, name string) int {
	for i, s := range steps {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// StepInput is what a handler receives.
type StepInput struct {
	InstanceID    uuid.UUID
	TransactionID uuid.UUID // uuid.Nil when workflow has no transaction
	Step          StepDef
	Attempt       int
	Compensating  bool // true when this is a reverse-direction execution
	Context       map[string]any
}

// StepOutput is merged into the instance context on success.
type StepOutput struct {
	Context map[string]any `json:"context,omitempty"`
}

// RetryableError marks transient failures — the engine retries with backoff
// up to StepDef.Retry. Any other error is terminal (on_failure policy runs).
type RetryableError struct{ Err error }

func (e RetryableError) Error() string { return e.Err.Error() }
func (e RetryableError) Unwrap() error { return e.Err }

func IsRetryable(err error) bool {
	var r RetryableError
	return errors.As(err, &r)
}

// StepHandler is implemented by every auto-step executor.
type StepHandler interface {
	Name() string
	Execute(ctx context.Context, in StepInput) (StepOutput, error)
}

// Events on the wire.
const (
	EventStepStart  = "workflow.step.start"
	EventStepResult = "workflow.step.result"
)

type StartEvent struct {
	InstanceID uuid.UUID `json:"instance_id"`
	Step       string    `json:"step"`
	Handler    string    `json:"handler"`
	Attempt    int       `json:"attempt"`
	// Compensate marks a reverse-direction execution (attempt >= 101 by
	// convention so it never collides with forward attempts).
	Compensate bool `json:"compensate,omitempty"`
}

type ResultEvent struct {
	InstanceID uuid.UUID       `json:"instance_id"`
	Step       string          `json:"step"`
	Attempt    int             `json:"attempt"`
	Compensate bool            `json:"compensate,omitempty"`
	OK         bool            `json:"ok"`
	Retryable  bool            `json:"retryable,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
}
