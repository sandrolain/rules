package models

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/sandrolain/rules/utils"
)

type Rule struct {
	Name            string
	Expression      string
	CompiledProgram cel.Program
}

func (r *Rule) BuildProgram(env *cel.Env) error {
	program, err := utils.BuildExpression(env, r.Expression, r.Name)
	if err != nil {
		return fmt.Errorf("error compiling rule %s: %v", r.Name, err)
	}
	r.CompiledProgram = program
	return nil
}

func (r *Rule) Evaluate(input map[string]interface{}) (RuleResult, error) {
	if r.CompiledProgram == nil {
		return RuleResult{}, fmt.Errorf("compiled program is nil")
	}

	out, _, err := r.CompiledProgram.Eval(map[string]interface{}{
		"input": input,
	})
	if err != nil {
		return RuleResult{}, err
	}

	switch value := out.Value().(type) {
	case int64:
		return RuleResult{
			Score:    value,
			Stop:     false,
			Passed:   true,
			Executed: true,
		}, nil
	case float64:
		return RuleResult{
			Score:    int64(value),
			Stop:     false,
			Passed:   true,
			Executed: true,
		}, nil
	case bool:
		return RuleResult{
			Score:    0,
			Stop:     false,
			Passed:   value,
			Executed: true,
		}, nil
	case map[string]interface{}:
		score, ok := value["value"].(int64)
		if !ok {
			if floatScore, ok := value["value"].(float64); ok {
				score = int64(floatScore)
			} else {
				return RuleResult{}, fmt.Errorf("invalid score value")
			}
		}
		stop, _ := value["stop"].(bool)
		return RuleResult{
			Score:    score,
			Stop:     stop,
			Passed:   true,
			Executed: true,
		}, nil
	default:
		return RuleResult{}, fmt.Errorf("unsupported result type")
	}
}

type RuleResult struct {
	Score    int64
	Stop     bool
	Passed   bool
	Executed bool
}
