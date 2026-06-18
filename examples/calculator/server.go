package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	core "low-entropy-core/go-core"
)

type GlobalStatus struct {
	Units    map[string]int    `json:"units"`
	Upgrades map[string]string `json:"upgrades"`
	Entropy  int               `json:"entropy"`
	LOC      int               `json:"loc"`
}

type VersionInfo struct {
	Dashboard       string `json:"dashboard"`
	Server          string `json:"server"`
	Types           string `json:"types"`
	CoreComposer    string `json:"core_composer"`
	CoreObservation string `json:"core_observation"`
	CoreHandoff     string `json:"core_handoff"`
	LastUpdated     string `json:"last_updated"`
	Notes           string `json:"notes"`
}

type FileChange struct {
	File      string `json:"file"`
	Change    string `json:"change"`
	Timestamp string `json:"timestamp"`
}

var (
	obs         = &core.InMemoryObservationAdapter{}
	fileChanges []FileChange
	filesMu     sync.Mutex
)

func addFileChange(fc FileChange) {
	filesMu.Lock()
	defer filesMu.Unlock()
	fileChanges = append(fileChanges, fc)
	if len(fileChanges) > 50 {
		fileChanges = fileChanges[len(fileChanges)-50:]
	}
}

func main() {
	// Load versions
	var versions VersionInfo
	data, _ := os.ReadFile("versions.json")
	json.Unmarshal(data, &versions)

	// ─── Build the Calculator Pipeline v2.0 ───
	calcPort := &CalculatorPort{}
	outputAdapter := &OutputAdapter{}
	historyAdapter := NewHistoryAdapter()

	calcPipeline := core.NewPipeline[Calculation](obs,
		core.PortAsStep(calcPort),
		core.AtomAsStep(TokenizeAtom()),
		core.AtomAsStep(ValidateAndPrepareAtom()),
		core.AtomAsStep(ToRPNAtom()),
		core.AtomAsStep(EvaluateRPNAtom()),
		core.AdapterAsStep(outputAdapter),
		core.AdapterAsStep(historyAdapter),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/api/global-status", func(w http.ResponseWriter, r *http.Request) {
		status := GlobalStatus{
			Units: map[string]int{
				"Composer": 12,
				"Port":     8,
				"Atom":     25,
				"Adapter":  15,
			},
			Upgrades: map[string]string{
				"Composer模式库":     "v2.0 泛型化 (Branch/Parallel/Retry/Timeout)",
				"ExecutionStep协议": "v2.0 UUID TraceID + TraceTree",
				"Agent Handoff":  "v2.0 SnapshotAdapter[T] + Transport",
			},
			Entropy: obs.StepCount(),
			LOC:     1200,
		}
		json.NewEncoder(w).Encode(status)
	})

	http.HandleFunc("/api/steps", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(obs.GetSteps())
	})

	http.HandleFunc("/api/trace-tree", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(obs.GetTraceTree())
	})

	http.HandleFunc("/api/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(versions)
	})

	http.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		filesMu.Lock()
		defer filesMu.Unlock()
		json.NewEncoder(w).Encode(fileChanges)
	})

	http.HandleFunc("/api/calculate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Expression string `json:"expression"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		ctx := context.Background()
		calc := Calculation{Expression: req.Expression, Success: true, Data: make(map[string]interface{})}

		result, _, err := calcPipeline.Run(ctx, calc)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"expression": req.Expression,
				"result":     0,
				"success":    false,
				"error":      err.Error(),
			})
			return
		}

		now := time.Now()
		addFileChange(FileChange{File: "examples/calculator/server.go", Change: "处理计算请求: " + req.Expression, Timestamp: now.Format(time.RFC3339)})

		json.NewEncoder(w).Encode(map[string]interface{}{
			"expression": req.Expression,
			"result":     result.Result,
			"success":    result.Success,
			"error":      result.ErrorMsg,
			"steps":      obs.GetSteps(),
		})
	})

	http.HandleFunc("/api/demo/handoff", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		now := time.Now()

		// Build a demo handoff pipeline
		scheduler := core.NewPipeline[any](obs,
			core.AtomAsStep(core.Atom[any, any](func(i any) any {
				fmt.Println("[Handoff Demo] Scheduler depositing state")
				return i
			})),
		)
		worker := core.NewPipeline[any](obs,
			core.AtomAsStep(core.Atom[any, any](func(i any) any {
				fmt.Println("[Handoff Demo] Worker withdrawing state")
				return i
			})),
		)

		snap := &core.DefaultSnapshotAdapter{}
		handoff := core.NewHandoff(scheduler, worker, snap, core.InProcTransport, obs)

		handoff.Run(ctx, core.HandoffRequest{
			SourceID: "calculator-scheduler",
			TargetID: "calculator-worker",
			TaskType: "calculation",
			Payload:  Calculation{Expression: "2+2", Success: true},
			Token:    "demo-handoff",
		})

		addFileChange(FileChange{File: "go-core/handoff.go", Change: "执行 Handoff 协议", Timestamp: now.Format(time.RFC3339)})
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "handoff triggered",
			"steps":  obs.GetSteps(),
		})
	})

	http.HandleFunc("/api/demo/pattern", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		now := time.Now()

		// Demo Branch pattern
		branch := core.NewBranch[Calculation](
			func(c Calculation) bool { return c.Expression != "" },
			core.NewPipeline[Calculation](obs,
				core.AtomAsStep(core.Atom[Calculation, Calculation](func(c Calculation) Calculation {
					fmt.Println("[Pattern Demo] Branch: true path")
					return c
				})),
			),
			core.NewPipeline[Calculation](obs,
				core.AtomAsStep(core.Atom[Calculation, Calculation](func(c Calculation) Calculation {
					fmt.Println("[Pattern Demo] Branch: false path")
					return c
				})),
			),
		)
		branch.Run(ctx, Calculation{Expression: "test", Success: true})

		// Demo Parallel pattern
		comp1 := core.NewPipeline[Calculation](obs,
			core.AtomAsStep(core.Atom[Calculation, Calculation](func(c Calculation) Calculation {
				fmt.Println("[Pattern Demo] Parallel: branch 1")
				return c
			})),
		)
		comp2 := core.NewPipeline[Calculation](obs,
			core.AtomAsStep(core.Atom[Calculation, Calculation](func(c Calculation) Calculation {
				fmt.Println("[Pattern Demo] Parallel: branch 2")
				return c
			})),
		)
		core.RunParallel[Calculation](ctx, Calculation{Expression: "parallel-test", Success: true}, comp1, comp2)

		addFileChange(FileChange{File: "go-core/composer.go", Change: "应用组合模式 (Branch + Parallel)", Timestamp: now.Format(time.RFC3339)})
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "patterns triggered",
			"steps":  obs.GetSteps(),
		})
	})

	fmt.Println("服务器启动在 :8083 (v2.0 泛型化架构)")
	http.ListenAndServe(":8083", nil)
}