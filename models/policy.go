package models

import (
	"github.com/google/cel-go/cel"
	"github.com/sandrolain/rules/utils"
)

type Policy struct {
	ID              string
	Name            string
	Rules           []Rule
	Expression      string // Changed from CELExpression
	CompiledProgram cel.Program
}

func (p *Policy) ShouldExecute(input map[string]interface{}) (bool, error) {
	if p.CompiledProgram == nil {
		return true, nil // If there's no expression, always execute the policy
	}

	out, _, err := p.CompiledProgram.Eval(map[string]interface{}{"input": input})
	if err != nil {
		return false, err
	}

	return out.Value().(bool), nil
}

func (p *Policy) AddRule(env *cel.Env, rule Rule) error {
	if rule.CompiledProgram == nil {
		program, err := utils.BuildExpression(env, rule.Expression, rule.Name)
		if err != nil {
			return err
		}

		rule.CompiledProgram = program
	}

	p.Rules = append(p.Rules, rule)
	return nil
}
