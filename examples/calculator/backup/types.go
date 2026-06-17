package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Calculation 领域模型
type Calculation struct {
	Expression string
	Result     float64
	Success    bool
	ErrorMsg   string
	Data       map[string]interface{}
}

// Atom: 解析表达式
func parseExpression(input interface{}) interface{} {
	calc := input.(Calculation)
	expr := strings.TrimSpace(calc.Expression)
	parts := strings.Fields(expr)
	if len(parts) != 3 {
		calc.Success = false
		calc.ErrorMsg = "格式错误，请使用: 数字 运算符 数字"
		return calc
	}
	num1, err1 := strconv.ParseFloat(parts[0], 64)
	op := parts[1]
	num2, err2 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil {
		calc.Success = false
		calc.ErrorMsg = "数字格式错误"
		return calc
	}
	calc.Data = map[string]interface{}{"num1": num1, "op": op, "num2": num2}
	return calc
}

// Atom: 执行计算
func performCalculation(input interface{}) interface{} {
	calc := input.(Calculation)
	if !calc.Success && calc.ErrorMsg != "" {
		return calc
	}
	data := calc.Data
	num1 := data["num1"].(float64)
	op := data["op"].(string)
	num2 := data["num2"].(float64)
	var result float64
	switch op {
	case "+":
		result = num1 + num2
	case "-":
		result = num1 - num2
	case "*":
		result = num1 * num2
	case "/":
		if num2 == 0 {
			calc.Success = false
			calc.ErrorMsg = "除数不能为零"
			return calc
		}
		result = num1 / num2
	default:
		calc.Success = false
		calc.ErrorMsg = "不支持的运算符: " + op
		return calc
	}
	calc.Result = result
	calc.Success = true
	return calc
}

// Port
type CalculatorPort struct{}

func (p *CalculatorPort) Call(input interface{}) interface{} {
	calc := input.(Calculation)
	expr := strings.TrimSpace(calc.Expression)
	if expr == "" {
		calc.Success = false
		calc.ErrorMsg = "表达式不能为空"
		return calc
	}
	parts := strings.Fields(expr)
	if len(parts) != 3 {
		calc.Success = false
		calc.ErrorMsg = "格式必须为: 数字 运算符 数字"
		return calc
	}
	calc.Success = true
	return calc
}

// Adapters
type OutputAdapter struct{}

func (o *OutputAdapter) PrintResult(input interface{}) interface{} {
	calc := input.(Calculation)
	if !calc.Success {
		fmt.Printf("[结果] 错误: %s\n", calc.ErrorMsg)
	} else {
		fmt.Printf("[结果] %s = %.2f\n", calc.Expression, calc.Result)
	}
	return calc
}

type HistoryAdapter struct {
	history []string
}

func (h *HistoryAdapter) Save(input interface{}) interface{} {
	calc := input.(Calculation)
	if calc.Success {
		entry := fmt.Sprintf("%s = %.2f", calc.Expression, calc.Result)
		h.history = append(h.history, entry)
		fmt.Printf("[历史] 已记录: %s\n", entry)
	}
	return calc
}
