// {{.Project}} — Port Implementations (Validation Gateways)
// Ports sit at system boundaries and validate input/output contracts.

package main

import (
	"context"
	"fmt"
)

// RequestPort validates incoming requests.
type RequestPort struct{}

func (p *RequestPort) Validate(ctx context.Context, req Request) (Request, error) {
	if req.Data == "" {
		return req, fmt.Errorf("data field is required")
	}
	if req.Action == "" {
		req.Action = "process"
	}
	if req.ID == "" {
		req.ID = "auto-generated"
	}
	return req, nil
}

// ResponsePort validates outgoing responses.
type ResponsePort struct{}

func (p *ResponsePort) Validate(ctx context.Context, resp Response) (Response, error) {
	if !resp.Success && resp.Error == "" {
		return resp, fmt.Errorf("failed response must have an error message")
	}
	return resp, nil
}
