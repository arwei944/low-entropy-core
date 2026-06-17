package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	
)

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

var globalHistory = NewHistoryAdapter()

func calculateHandler(w http.ResponseWriter, r *http.Request) {
	var req CalcRequest
	json.NewDecoder(r.Body).Decode(&req)

	steps := []ExecutionStep{}

	if req.Expression == "MR" {
		resp := CalcResponse{
			Expression: "MR",
			Result:     memory,
			Success:    true,
			Steps: []ExecutionStep{
				{Unit: "Adapter", Action: "Memory", Details: "MR - 读取 memory = " + fmt.Sprintf("%.6g", memory)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
	if req.Expression == "MC" {
		memory = 0
		resp := CalcResponse{
			Expression: "MC",
			Result:     0,
			Success:    true,
			Steps: []ExecutionStep{
				{Unit: "Adapter", Action: "Memory", Details: "MC - 清空 memory"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	calc := Calculation{Expression: req.Expression, Success: true, Data: map[string]interface{}{}}

	steps = append(steps, ExecutionStep{Unit: "Composer", Action: "Pipeline启动", Details: "开始完整表达式计算编排"})

	port := &CalculatorPort{}
	calc = port.Call(calc).(Calculation)
	steps = append(steps, ExecutionStep{Unit: "Port", Action: "Validate", Details: "验证表达式非空及格式"})

	if !calc.Success {
		resp := CalcResponse{
			Expression: req.Expression,
			Result:     0,
			Success:    false,
			ErrorMsg:   calc.ErrorMsg,
			Steps:      steps,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	calc = tokenize(calc).(Calculation)
	steps = append(steps, ExecutionStep{Unit: "Atom", Action: "tokenize", Details: "词法分析生成 tokens"})

	calc = validateAndPrepare(calc).(Calculation)
	steps = append(steps, ExecutionStep{Unit: "Atom", Action: "validateAndPrepare", Details: "括号匹配与准备"})

	if !calc.Success {
		resp := CalcResponse{Expression: req.Expression, Result: 0, Success: false, ErrorMsg: calc.ErrorMsg, Steps: steps}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	calc = toRPN(calc).(Calculation)
	steps = append(steps, ExecutionStep{Unit: "Atom", Action: "toRPN", Details: "转换为逆波兰表示法 (支持优先级、^ 和 %)"})

	calc = evaluateRPN(calc).(Calculation)
	if calc.Success {
		steps = append(steps, ExecutionStep{Unit: "Atom", Action: "evaluateRPN", Details: fmt.Sprintf("RPN 计算结果 = %.6g", calc.Result)})
	} else {
		steps = append(steps, ExecutionStep{Unit: "Atom", Action: "evaluateRPN", Details: "计算失败: " + calc.ErrorMsg})
	}

	_ = (&OutputAdapter{}).PrintResult(calc)
	steps = append(steps, ExecutionStep{Unit: "Adapter", Action: "Output", Details: "输出结果到控制台"})

	_ = globalHistory.Save(calc)
	steps = append(steps, ExecutionStep{Unit: "Adapter", Action: "History", Details: "保存到文件 history.txt (持久化)"})

	resp := CalcResponse{
		Expression: req.Expression,
		Result:     calc.Result,
		Success:    calc.Success,
		ErrorMsg:   calc.ErrorMsg,
		Steps:      steps,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"history": globalHistory.history})
}

func clearHistoryHandler(w http.ResponseWriter, r *http.Request) {
	globalHistory.Clear()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func main() {
	http.HandleFunc("/api/calculate", calculateHandler)
	http.HandleFunc("/api/history", historyHandler)
	http.HandleFunc("/api/clear-history", clearHistoryHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	fmt.Println("=== 低熵完整计算器 (完整版) 已启动 on :8081 ===")
	fmt.Println("访问 http://localhost:8081 查看专业计算器 + 实时4单元 + 文件持久化历史 + 监控")
	http.ListenAndServe(":8081", nil)
}