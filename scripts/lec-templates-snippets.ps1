# lec-templates-snippets.ps1 — Primitive Snippet Generators
# Part of lec — Low-Entropy Core CLI v0.3.0

function Generate-AtomSnippet {
    param([string]$Name, [string]$CoreMod)
    return @"
// $Name — Atom (Pure Computation)
// Atoms have NO side effects. Same input always produces same output.

package main

import (
	core "$CoreMod"
)

func $Name() core.Atom[Request, Response] {
	return core.Atom[Request, Response](func(req Request) Response {
		// TODO: Implement pure computation logic here
		return Response{
			Success: true,
		}
	})
}
"@
}

function Generate-PortSnippet {
    param([string]$Name, [string]$CoreMod)
    return @"
// $Name — Port (Validation Gateway)
// Ports validate input/output at system boundaries.

package main

import (
	"context"
	"fmt"
)

type $Name struct{}

func (p *$Name) Validate(ctx context.Context, req Request) (Request, error) {
	// TODO: Implement validation logic here
	if req.Data == "" {
		return req, fmt.Errorf("validation failed: empty data")
	}
	return req, nil
}
"@
}

function Generate-AdapterSnippet {
    param([string]$Name, [string]$CoreMod)
    return @"
// $Name — Adapter (Side-Effect Boundary)
// Adapters are the ONLY place where side effects are allowed.

package main

import (
	"context"
	"fmt"
)

type $Name struct{}

func (a *$Name) Execute(ctx context.Context, resp Response) (Response, error) {
	// TODO: Implement side-effect logic here (I/O, DB, external API)
	fmt.Printf("[$Name] executed\n")
	return resp, nil
}
"@
}

function Generate-ComposerSnippet {
    param([string]$Name, [string]$CoreMod)
    return @"
// $Name — Composer (Orchestration)
// Composers orchestrate multiple Steps into a pipeline.

package main

import (
	core "$CoreMod"
)

func $Name(obs core.ObservationAdapter) core.Composer[Request] {
	return core.NewPipeline[Request](obs,
		// TODO: Add steps here
		// core.PortAsStep(&SomePort{}),
		// core.AtomAsStep(SomeAtom()),
		// core.AdapterAsStep(&SomeAdapter{}),
	)
}
"@
}
