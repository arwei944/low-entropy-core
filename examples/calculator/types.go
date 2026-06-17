package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"

	core "low-entropy-core/go-core"
)

// Calculation carries the state through the pipeline.
type Calculation struct {
	Expression string
	Result     float64
	Success    bool
	ErrorMsg   string
	Data       map[string]interface{}
}

// ─── Atom: pure functions (no side effects) ───

// TokenizeAtom breaks an expression string into tokens.
func TokenizeAtom() core.Atom[Calculation, Calculation] {
	return func(calc Calculation) Calculation {
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
}

// ValidateAndPrepareAtom validates bracket matching.
func ValidateAndPrepareAtom() core.Atom[Calculation, Calculation] {
	return func(calc Calculation) Calculation {
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
}

// ToRPNAtom converts infix tokens to Reverse Polish Notation.
func ToRPNAtom() core.Atom[Calculation, Calculation] {
	return func(calc Calculation) Calculation {
		if !calc.Success {
			return calc
		}
		tokens := calc.Data["tokens"].([]string)
		var output []string
		var stack []string
		precedence := map[string]int{"+": 1, "-": 1, "*": 2, "/": 2, "^": 3, "%": 2}
		rightAssoc := map[string]bool{"^": true}
		for _, token := range tokens {
			if isNumberStr(token) {
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
			output = append(output, stack[len(stack)-1])
			stack = stack[:len(stack)-1]
		}
		calc.Data["rpn"] = output
		return calc
	}
}

func isNumberStr(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// EvaluateRPNAtom evaluates the RPN expression.
func EvaluateRPNAtom() core.Atom[Calculation, Calculation] {
	return func(calc Calculation) Calculation {
		if !calc.Success {
			return calc
		}
		rpn := calc.Data["rpn"].([]string)
		var stack []float64
		for _, token := range rpn {
			if isNumberStr(token) {
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
}

// ─── Port: contract validation ───

// CalculatorPort validates the input expression.
type CalculatorPort struct{}

func (p *CalculatorPort) Validate(ctx context.Context, input Calculation) (Calculation, error) {
	if strings.TrimSpace(input.Expression) == "" {
		return input, &core.StepError{Code: "EMPTY_EXPRESSION", Message: "表达式不能为空", Recoverable: false}
	}
	input.Success = true
	return input, nil
}

// ─── Adapter: side effects (output, history) ───

// OutputAdapter prints the result to stdout.
type OutputAdapter struct{}

func (a *OutputAdapter) Execute(ctx context.Context, input Calculation) (Calculation, error) {
	if !input.Success {
		fmt.Printf("[Adapter Output] 错误: %s\n", input.ErrorMsg)
	} else {
		fmt.Printf("[Adapter Output] %s = %.6g\n", input.Expression, input.Result)
	}
	return input, nil
}

// HistoryAdapter persists calculation history to a file.
type HistoryAdapter struct {
	history []string
	file    string
}

func NewHistoryAdapter() *HistoryAdapter {
	h := &HistoryAdapter{file: "history.txt"}
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

func (h *HistoryAdapter) Execute(ctx context.Context, input Calculation) (Calculation, error) {
	if input.Success {
		entry := fmt.Sprintf("%s = %.6g", input.Expression, input.Result)
		h.history = append(h.history, entry)
		f, err := os.OpenFile(h.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			fmt.Fprintln(f, entry)
			f.Close()
		}
		fmt.Printf("[Adapter History] 已保存到文件: %s\n", entry)
	}
	return input, nil
}

func (h *HistoryAdapter) Clear() {
	h.history = []string{}
	os.Remove(h.file)
	fmt.Println("[Adapter History] 历史已清空 (文件删除)")
}