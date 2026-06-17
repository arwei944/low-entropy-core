package core

// Adapter is the third basic unit: side effects only
// (Persistence, logging, I/O)
type Adapter interface {
	Execute(interface{}) interface{}
}
