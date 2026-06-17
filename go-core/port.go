package core

// Port is the second basic unit: contract / validation
type Port interface {
	Call(interface{}) interface{}
}
