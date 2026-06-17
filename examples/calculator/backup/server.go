package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"low-entropy-core/go-core"
)

// ExecutionStep 后端执行步骤（供前端实时展示）
type ExecutionStep struct {
	Unit    string `json:"unit"`
	Action  string `json:"action"`
	Details string `json:"details"`
}

type CalcRequest struct {
	Expression string `json:"expression"`
}

type CalcResponse struct {
	Expression string          `json:"expression"`
	Result     float64         `json:"result"`
	Success    bool            `json:"success"`
	ErrorMsg   string          `json:"error_msg"`
	Steps      []ExecutionStep `json:"steps"`
}

var globalHistory = &HistoryAdapter{}

func calculateHandler(w http.ResponseWriter, r *http.Request) {
	var req CalcRequest
	json.NewDecoder(r.Body).Decode(&req)

	steps := []ExecutionStep{}
	calc := Calculation{Expression: req.Expression, Data: map[string]interface{}{}}

	composer := core.NewPipeline(
		func(i interface{}) interface{} {
			steps = append(steps, ExecutionStep{Unit: "Composer", Action: "Pipeline", Details: "启动计算编排"})
			
			port := &CalculatorPort{}
			res := port.Call(i)
			steps = append(steps, ExecutionStep{Unit: "Port", Action: "Validate", Details: "验证表达式格式"})
			return res
		},
		func(i interface{}) interface{} {
			res := parseExpression(i)
			steps = append(steps, ExecutionStep{Unit: "Atom", Action: "parseExpression", Details: "解析数字与运算符"})
			return res
		},
		func(i interface{}) interface{} {
			res := performCalculation(i)
			c := res.(Calculation)
			if c.Success {
				steps = append(steps, ExecutionStep{Unit: "Atom", Action: "performCalculation", Details: fmt.Sprintf("计算结果 = %.2f", c.Result)})
			} else {
				steps = append(steps, ExecutionStep{Unit: "Atom", Action: "performCalculation", Details: "计算失败"})
			}
			return res
		},
		func(i interface{}) interface{} {
			res := (&OutputAdapter{}).PrintResult(i)
			steps = append(steps, ExecutionStep{Unit: "Adapter", Action: "Output", Details: "返回结果"})
			return res
		},
		func(i interface{}) interface{} {
			res := globalHistory.Save(i)
			steps = append(steps, ExecutionStep{Unit: "Adapter", Action: "History", Details: "记录历史"})
			return res
		},
	)

	final := composer.Run(calc).(Calculation)

	resp := CalcResponse{
		Expression: req.Expression,
		Result:     final.Result,
		Success:    final.Success,
		ErrorMsg:   final.ErrorMsg,
		Steps:      steps,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"history": globalHistory.history})
}

func main() {
	http.HandleFunc("/api/calculate", calculateHandler)
	http.HandleFunc("/api/history", historyHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	fmt.Println("低熵计算器 Web 版已启动")
	fmt.Println("访问: http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
