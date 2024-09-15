package models

import (
	"fmt"
	"sort"

	"github.com/google/cel-go/cel"
	"github.com/sandrolain/rules/utils"
)

type Threshold struct {
	ID    string
	Value int64 // Cambiato da int a int64
}

type Policy struct {
	ID              string
	Name            string
	Expression      string
	Rules           []Rule
	Thresholds      []Threshold
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

func (p *Policy) Evaluate(input map[string]interface{}) (string, []RuleResult, error) {
	var totalScore int64
	ruleResults := make([]RuleResult, len(p.Rules))
	stopped := false

	for i, rule := range p.Rules {
		if stopped {
			ruleResults[i] = RuleResult{Executed: false}
			continue
		}

		result, err := rule.Evaluate(input)
		if err != nil {
			return "", nil, fmt.Errorf("error evaluating rule %s: %v", rule.Name, err)
		}

		result.Executed = true
		ruleResults[i] = result
		totalScore += result.Score

		if result.Stop {
			stopped = true
		}
	}

	return p.getThresholdID(totalScore), ruleResults, nil
}

func (p *Policy) getThresholdID(score int64) string { // Cambiato da int a int64
	sort.Slice(p.Thresholds, func(i, j int) bool {
		return p.Thresholds[i].Value < p.Thresholds[j].Value
	})

	for i := len(p.Thresholds) - 1; i >= 0; i-- {
		if score >= p.Thresholds[i].Value {
			return p.Thresholds[i].ID
		}
	}

	return ""
}
