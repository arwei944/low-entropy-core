package core

import (
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// ArchitectureRegistry — static architecture view
// ──────────────────────────────────────────────

// PipelineDescriptor describes a registered pipeline in the architecture.
type PipelineDescriptor struct {
	// Name is the human-readable name of the pipeline.
	Name string `json:"name"`

	// Description explains what the pipeline does.
	Description string `json:"description"`

	// StepCount is the number of steps in the pipeline.
	StepCount int `json:"step_count"`

	// Patterns are the patterns used by this pipeline.
	Patterns []string `json:"patterns"`

	// PortContracts are the contracts enforced by this pipeline's ports.
	PortContracts []string `json:"port_contracts,omitempty"`

	// RegisteredAt is when this pipeline was registered.
	RegisteredAt time.Time `json:"registered_at"`
}

// ArchitectureRegistry maintains a static view of all registered pipelines,
// their patterns, and their port contracts. It provides the static anatomy
// perspective for the X-Ray dashboard.
type ArchitectureRegistry struct {
	mu        sync.RWMutex
	pipelines map[string]*PipelineDescriptor
	contracts map[string]PortContract
}

// NewArchitectureRegistry creates a new architecture registry.
func NewArchitectureRegistry() *ArchitectureRegistry {
	return &ArchitectureRegistry{
		pipelines: make(map[string]*PipelineDescriptor),
		contracts: make(map[string]PortContract),
	}
}

// Register records a pipeline descriptor in the registry.
func (r *ArchitectureRegistry) Register(desc PipelineDescriptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	desc.RegisteredAt = time.Now()
	r.pipelines[desc.Name] = &desc
}

// RegisterContract records a port contract in the registry.
func (r *ArchitectureRegistry) RegisterContract(contract PortContract) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contracts[contract.Name] = contract
}

// Get retrieves a pipeline descriptor by name.
func (r *ArchitectureRegistry) Get(name string) (*PipelineDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.pipelines[name]
	return p, ok
}

// GetContract retrieves a port contract by name.
func (r *ArchitectureRegistry) GetContract(name string) (PortContract, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.contracts[name]
	return c, ok
}

// List returns all registered pipeline descriptors.
func (r *ArchitectureRegistry) List() []PipelineDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]PipelineDescriptor, 0, len(r.pipelines))
	for _, p := range r.pipelines {
		result = append(result, *p)
	}
	return result
}

// ListContracts returns all registered port contracts.
func (r *ArchitectureRegistry) ListContracts() []PortContract {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]PortContract, 0, len(r.contracts))
	for _, c := range r.contracts {
		result = append(result, c)
	}
	return result
}

// Count returns the number of registered pipelines.
func (r *ArchitectureRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.pipelines)
}

// ContractCount returns the number of registered contracts.
func (r *ArchitectureRegistry) ContractCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.contracts)
}