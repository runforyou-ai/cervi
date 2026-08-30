//go:build server

package agentruntime

import (
	"context"
	"errors"
	"math"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

type calculatorInput struct {
	Operation string  `json:"operation" jsonschema:"required,enum=add,enum=subtract,enum=multiply,enum=divide" jsonschema_description:"Arithmetic operation to perform"`
	Left      float64 `json:"left" jsonschema:"required" jsonschema_description:"Left operand"`
	Right     float64 `json:"right" jsonschema:"required" jsonschema_description:"Right operand"`
}

type calculatorOutput struct {
	Result float64 `json:"result"`
}

// newCalculatorTool 创建四则运算 Tool。
func newCalculatorTool() (tool.InvokableTool, error) {
	return toolutils.InferTool(
		"calculator",
		"Perform addition, subtraction, multiplication, or division on two numbers.",
		calculate,
	)
}

// calculate 执行一次四则运算并拒绝无效结果。
func calculate(_ context.Context, input calculatorInput) (calculatorOutput, error) {
	var result float64
	switch input.Operation {
	case "add":
		result = input.Left + input.Right
	case "subtract":
		result = input.Left - input.Right
	case "multiply":
		result = input.Left * input.Right
	case "divide":
		if input.Right == 0 {
			return calculatorOutput{}, errors.New("cannot divide by zero")
		}
		result = input.Left / input.Right
	default:
		return calculatorOutput{}, errors.New("unsupported calculator operation")
	}
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return calculatorOutput{}, errors.New("calculator result is not finite")
	}
	return calculatorOutput{Result: result}, nil
}
