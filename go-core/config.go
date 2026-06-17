package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────
// Pipeline Configuration Types
// ──────────────────────────────────────────────

// AllowedStepTypes defines the valid step type values for pipeline configuration.
// These correspond to the four primitives: Atom, Port, Adapter, Composer.
var AllowedStepTypes = []string{"atom", "port", "adapter", "composer"}

// PipelineConfig defines the configuration for a pipeline.
type PipelineConfig struct {
	ID    string       `json:"id"`    // unique pipeline identifier
	Name  string       `json:"name"`  // human-readable name
	Steps []StepConfig `json:"steps"` // ordered list of step configurations
}

// StepConfig defines the configuration for a single step in a pipeline.
type StepConfig struct {
	Type   string         `json:"type"`   // "atom", "port", "adapter", "composer"
	Name   string         `json:"name"`   // step name for identification
	Params map[string]any `json:"params"` // step-specific parameters
}

// ──────────────────────────────────────────────
// Configuration Parsing & Validation
// ──────────────────────────────────────────────

// isAllowedStepType checks whether the given type string is one of the allowed step types.
func isAllowedStepType(t string) bool {
	for _, allowed := range AllowedStepTypes {
		if t == allowed {
			return true
		}
	}
	return false
}

// ParseConfig parses a JSON byte slice into a PipelineConfig.
// Returns an error if:
//   - JSON is invalid
//   - ID is empty
//   - Type is not one of "atom", "port", "adapter", "composer"
//   - Steps array is empty
func ParseConfig(jsonBytes []byte) (*PipelineConfig, error) {
	if len(jsonBytes) == 0 {
		return nil, fmt.Errorf("config: empty JSON input")
	}

	var config PipelineConfig
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return nil, fmt.Errorf("config: invalid JSON: %w", err)
	}

	if config.ID == "" {
		return nil, fmt.Errorf("config: pipeline ID must not be empty")
	}

	if len(config.Steps) == 0 {
		return nil, fmt.Errorf("config: pipeline %q has no steps defined", config.ID)
	}

	for i, step := range config.Steps {
		if !isAllowedStepType(step.Type) {
			return nil, fmt.Errorf(
				"config: step %d (%q) has invalid type %q; must be one of [%s]",
				i, step.Name, step.Type, strings.Join(AllowedStepTypes, ", "),
			)
		}
	}

	return &config, nil
}

// ValidateConfig validates a PipelineConfig and returns all errors found.
// This is a non-destructive validation that checks the entire config
// and collects every issue before returning.
func ValidateConfig(config *PipelineConfig) []error {
	var errs []error

	if config == nil {
		return append(errs, fmt.Errorf("config: PipelineConfig is nil"))
	}

	if config.ID == "" {
		errs = append(errs, fmt.Errorf("config: pipeline ID must not be empty"))
	}

	if len(config.Steps) == 0 {
		errs = append(errs, fmt.Errorf("config: pipeline %q has no steps defined", config.ID))
	}

	for i, step := range config.Steps {
		if step.Name == "" {
			errs = append(errs, fmt.Errorf("config: step %d has an empty name", i))
		}
		if !isAllowedStepType(step.Type) {
			errs = append(errs, fmt.Errorf(
				"config: step %d (%q) has invalid type %q; must be one of [%s]",
				i, step.Name, step.Type, strings.Join(AllowedStepTypes, ", "),
			))
		}
	}

	return errs
}

// ParseAndValidateConfig parses JSON and validates the result in one step.
// It returns the parsed config, any validation errors, and a parse error if
// the JSON itself is invalid.
//
// If the JSON is invalid, config and errs will both be nil; only the parseErr
// is set.
// If the JSON is valid but the config fails validation, config is still returned
// alongside the validation errors.
func ParseAndValidateConfig(jsonBytes []byte) (*PipelineConfig, []error, error) {
	config, parseErr := ParseConfig(jsonBytes)
	if parseErr != nil {
		return nil, nil, parseErr
	}

	errs := ValidateConfig(config)
	return config, errs, nil
}