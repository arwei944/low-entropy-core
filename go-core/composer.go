package core

// Composer is the 4th unit for orchestration
type Composer struct {
	steps []func(interface{}) interface{}
}

// NewPipeline creates a Composer using Pipeline mode (chains functions)
func NewPipeline(funcs ...func(interface{}) interface{}) *Composer {
	return &Composer{steps: funcs}
}

// Run executes the pipeline
func (c *Composer) Run(input interface{}) interface{} {
	result := input
	for _, step := range c.steps {
		result = step(result)
	}
	return result
}
