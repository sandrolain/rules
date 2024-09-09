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

func (r *Rule) Evaluate(input map[string]interface{}) (bool, error) {
	if r.CompiledProgram == nil {
		return false, fmt.Errorf("compiled program is nil")
	}

	result, _, err := r.CompiledProgram.Eval(map[string]interface{}{"input": input})
	if err != nil {
		return false, err
	}

	return result.Value().(bool), nil
}
