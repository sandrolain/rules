package models

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/sandrolain/rules/utils"
	"github.com/stretchr/testify/assert"
)

func TestPolicy_ShouldExecute(t *testing.T) {
	tests := []struct {
		name           string
		policy         Policy
		input          map[string]interface{}
		expectedResult bool
		expectError    bool
	}{
		{
			name: "Policy with no expression should always execute",
			policy: Policy{
				Name:            "AlwaysExecute",
				Expression:      "",
				CompiledProgram: nil,
			},
			input:          map[string]interface{}{"key": "value"},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Policy with true expression should execute",
			policy: Policy{
				Name:            "TrueExpression",
				Expression:      "true",
				CompiledProgram: mustCompileProgram(t, "true"),
			},
			input:          map[string]interface{}{"key": "value"},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Policy with false expression should not execute",
			policy: Policy{
				Name:            "FalseExpression",
				Expression:      "false",
				CompiledProgram: mustCompileProgram(t, "false"),
			},
			input:          map[string]interface{}{"key": "value"},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Policy with input-dependent expression (true case)",
			policy: Policy{
				Name:            "InputDependentTrue",
				Expression:      "input.key == 'execute'",
				CompiledProgram: mustCompileProgram(t, "input.key == 'execute'"),
			},
			input:          map[string]interface{}{"key": "execute"},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Policy with input-dependent expression (false case)",
			policy: Policy{
				Name:            "InputDependentFalse",
				Expression:      "input.key == 'execute'",
				CompiledProgram: mustCompileProgram(t, "input.key == 'execute'"),
			},
			input:          map[string]interface{}{"key": "do not execute"},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Policy with complex expression",
			policy: Policy{
				Name:            "ComplexExpression",
				Expression:      "input.age >= 18 && input.country in ['US', 'CA', 'UK']",
				CompiledProgram: mustCompileProgram(t, "input.age >= 18 && input.country in ['US', 'CA', 'UK']"),
			},
			input:          map[string]interface{}{"age": 25, "country": "US"},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Policy with invalid input type",
			policy: Policy{
				Name:            "InvalidInputType",
				Expression:      "input.age > 18",
				CompiledProgram: mustCompileProgram(t, "input.age > 18"),
			},
			input:          map[string]interface{}{"age": "not a number"},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.policy.ShouldExecute(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPolicy_AddRule(t *testing.T) {
	env, err := cel.NewEnv(cel.Declarations(
		decls.NewVar("input", decls.NewMapType(decls.String, decls.Any)),
	))
	assert.NoError(t, err)

	policy := Policy{
		Name:       "TestPolicy",
		Expression: "true",
	}

	tests := []struct {
		name        string
		rule        Rule
		expectError bool
	}{
		{
			name: "Add valid rule",
			rule: Rule{
				Name:       "ValidRule",
				Expression: "input.value > 10",
			},
			expectError: false,
		},
		{
			name: "Add rule with invalid expression",
			rule: Rule{
				Name:       "InvalidRule",
				Expression: "this is not a valid expression",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.AddRule(env, tt.rule)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, policy.Rules, 1)
				assert.Equal(t, tt.rule.Name, policy.Rules[0].Name)
				assert.Equal(t, tt.rule.Expression, policy.Rules[0].Expression)
				assert.NotNil(t, policy.Rules[0].CompiledProgram)
			}
		})

		// Clear rules after each test
		policy.Rules = nil
	}
}

func mustCompileProgram(t *testing.T, expression string) cel.Program {
	t.Helper()
	env, err := cel.NewEnv(cel.Declarations(
		decls.NewVar("input", decls.NewMapType(decls.String, decls.Any)),
	))
	assert.NoError(t, err)

	program, err := utils.BuildExpression(env, expression, "test")
	assert.NoError(t, err)

	return program
}
