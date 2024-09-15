package models

import (
	"testing"

	"github.com/sandrolain/rules/cel"
	"github.com/stretchr/testify/assert"
)

func TestRule_BuildProgram(t *testing.T) {
	env, err := cel.CreateRuleEnv()
	assert.NoError(t, err)

	tests := []struct {
		name        string
		rule        Rule
		expectError bool
	}{
		{
			name: "Valid rule",
			rule: Rule{
				Name:       "ValidRule",
				Expression: "Result(10, true)",
			},
			expectError: false,
		},
		{
			name: "Invalid rule",
			rule: Rule{
				Name:       "InvalidRule",
				Expression: "this is not a valid expression",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.BuildProgram(env)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tt.rule.CompiledProgram)
			}
		})
	}
}

func TestRule_Evaluate(t *testing.T) {
	env, err := cel.CreateRuleEnv()
	assert.NoError(t, err)

	tests := []struct {
		name           string
		rule           Rule
		input          map[string]interface{}
		expectedResult RuleResult
		expectError    bool
	}{
		{
			name: "Simple rule",
			rule: Rule{
				Name:       "SimpleRule",
				Expression: "Result(10, false)",
			},
			input:          map[string]interface{}{},
			expectedResult: RuleResult{Score: 10, Stop: false, Passed: true, Executed: true},
			expectError:    false,
		},
		{
			name: "Input-dependent rule",
			rule: Rule{
				Name:       "InputRule",
				Expression: "Result(input.value * 2, input.value > 10)",
			},
			input:          map[string]interface{}{"value": 15},
			expectedResult: RuleResult{Score: 30, Stop: true, Passed: true, Executed: true},
			expectError:    false,
		},
		{
			name: "Invalid rule",
			rule: Rule{
				Name:       "InvalidRule",
				Expression: "this will not compile",
			},
			input:       map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.BuildProgram(env)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			result, err := tt.rule.Evaluate(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
