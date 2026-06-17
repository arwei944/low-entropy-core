package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Calculation struct {
	Expression string
	Result     float64
	Success    bool
	ErrorMsg   string
	Data       map[string]interface{}
}

var memory float64 = 0

func tokenize(input interface{}) interface{} {
	calc := input.(Calculation)
	expr := strings.ReplaceAll(calc.Expression, " ", "")
	tokens := []string{}
	i := 0
	for i < len(expr) {
		c := rune(expr[i])
		if unicode.IsDigit(c) || c == '.' {
			j := i
			for j < len(expr) && (unicode.IsDigit(rune(expr[j])) || expr[j] == '.') {
				j++
			}
			tokens = append(tokens, expr[i:j])
			i = j
		} else if c == '+' || c == '-' || c == '*' || c == '/' || c == '^' || c == '%' || c == '(' || c == ')' {
			tokens = append(tokens, string(c))
			i++
		} else {
			calc.Success = false
			calc.ErrorMsg = "非法字符: " + string(c)
			return calc
		}
	}
	calc.Data = map[string]interface{}{"tokens": tokens}
	return calc
}

func validateAndPrepare(input interface{}) interface{} {
	calc := input.(Calculation)
	if !calc.Success {
		return calc
	}
	tokens := calc.Data["tokens"].([]string)
	depth := 0
	for _, t := range tokens {
		if t == "(" {
			depth++
		} else if t == ")" {
			depth--
			if depth < 0 {
				calc.Success = false
				calc.ErrorMsg = "括号不匹配"
				return calc
			}
		}
	}
	if depth != 0 {
		calc.Success = false
		calc.ErrorMsg = "括号不匹配"
		return calc
	}
	calc.Success = true
	return calc
}

func toRPN(input interface{}) interface{} {
	calc := input.(Calculation)
	if !calc.Success {
		return calc
	}
	tokens := calc.Data["tokens"].([]string)
	var output []string
	var stack []string
	precedence := map[string]int{"+": 1, "-": 1, "*": 2, "/": 2, "^": 3, "%": 2}
	rightAssoc := map[string]bool{"^": true}
	for _, token := range tokens {
		if isNumber(token) {
			output = append(output, token)
		} else if token == "(" {
			stack = append(stack, token)
		} else if token == ")" {
			for len(stack) > 0 && stack[len(stack)-1] != "(" {
				output = append(output, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		} else {
			for len(stack) > 0 {
				top := stack[len(stack)-1]
				if top == "(" {
					break
				}
				p1 := precedence[token]
				p2 := precedence[top]
				if (p2 > p1) || (p2 == p1 && !rightAssoc[token]) {
					output = append(output, top)
					stack = stack[:len(stack)-1]
				} else {
					break
				}
			}
			stack = append(stack, token)
		}
	}
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		output = append(output, top)
		stack = stack[:len(stack)-1]
	}
	calc.Data["rpn"] = output
	return calc
}

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func evaluateRPN(input interface{}) interface{} {
	calc := input.(Calculation)
	if !calc.Success {
		return calc
	}
	rpn := calc.Data["rpn"].([]string)
	var stack []float64
	for _, token := range rpn {
		if isNumber(token) {
			f, _ := strconv.ParseFloat(token, 64)
			stack = append(stack, f)
		} else {
			if len(stack) < 2 {
				calc.Success = false
				calc.ErrorMsg = "表达式不完整"
				return calc
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			var res float64
			switch token {
			case "+":
				res = a + b
			case "-":
				res = a - b
			case "*":
				res = a * b
			case "/":
				if b == 0 {
					calc.Success = false
					calc.ErrorMsg = "除数不能为零"
					return calc
				}
				res = a / b
			case "^":
				res = math.Pow(a, b)
			case "%":
				if b == 0 {
					calc.Success = false
					calc.ErrorMsg = "模数不能为零"
					return calc
				}
				res = math.Mod(a, b)
			default:
				calc.Success = false
				calc.ErrorMsg = "未知运算符"
				return calc
			}
			stack = append(stack, res)
		}
	}
	if len(stack) != 1 {
		calc.Success = false
		calc.ErrorMsg = "表达式计算错误"
		return calc
	}
	calc.Result = stack[0]
	calc.Success = true
	return calc
}

type CalculatorPort struct{}

func (p *CalculatorPort) Call(input interface{}) interface{} {
	calc := input.(Calculation)
	if strings.TrimSpace(calc.Expression) == "" {
		calc.Success = false
		calc.ErrorMsg = "表达式不能为空"
		return calc
	}
	calc.Success = true
	return calc
}

type OutputAdapter struct{}

func (o *OutputAdapter) PrintResult(input interface{}) interface{} {
	calc := input.(Calculation)
	if !calc.Success {
		fmt.Printf("[Adapter Output] 错误: %s\n", calc.ErrorMsg)
	} else {
		fmt.Printf("[Adapter Output] %s = %.6g\n", calc.Expression, calc.Result)
	}
	return calc
}

type HistoryAdapter struct {
	history []string
	file    string
}

func NewHistoryAdapter() *HistoryAdapter {
	h := &HistoryAdapter{file: "history.txt"}
	// 加载已有历史 (低熵 append-only 文件)
	if data, err := os.ReadFile(h.file); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				h.history = append(h.history, line)
			}
		}
	}
	return h
}

func (h *HistoryAdapter) Save(input interface{}) interface{} {
	calc := input.(Calculation)
	if calc.Success {
		entry := fmt.Sprintf("%s = %.6g", calc.Expression, calc.Result)
		h.history = append(h.history, entry)
		// 文件持久化 (Adapter 唯一副作用)
		f, err := os.OpenFile(h.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			fmt.Fprintln(f, entry)
			f.Close()
		}
		fmt.Printf("[Adapter History] 已保存到文件: %s\n", entry)
	}
	return calc
}

func (h *HistoryAdapter) Clear() {
	h.history = []string{}
	os.Remove(h.file)
	fmt.Println("[Adapter History] 历史已清空 (文件删除)")
}