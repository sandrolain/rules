package models

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	celext "github.com/google/cel-go/ext" // Aggiungi questa riga
	"github.com/stretchr/testify/assert"
)

func TestRule(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("input", decls.NewMapType(decls.String, decls.Any)),
		),
		celext.Strings(), // Aggiungi questa riga
	)
	assert.NoError(t, err)

	tests := []struct {
		name           string
		rule           Rule
		input          map[string]interface{}
		expectedResult bool
		expectError    bool
	}{
		{
			name: "Simple true condition",
			rule: Rule{
				Name:       "SimpleTrue",
				Expression: "true",
			},
			input:          map[string]interface{}{},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Simple false condition",
			rule: Rule{
				Name:       "SimpleFalse",
				Expression: "false",
			},
			input:          map[string]interface{}{},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Input-dependent condition (true case)",
			rule: Rule{
				Name:       "InputDependent",
				Expression: "input.age >= 18",
			},
			input:          map[string]interface{}{"age": 20},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Input-dependent condition (false case)",
			rule: Rule{
				Name:       "InputDependent",
				Expression: "input.age >= 18",
			},
			input:          map[string]interface{}{"age": 16},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Complex condition",
			rule: Rule{
				Name:       "Complex",
				Expression: "input.age >= 18 && input.country in ['US', 'CA', 'UK']",
			},
			input:          map[string]interface{}{"age": 25, "country": "US"},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Invalid expression",
			rule: Rule{
				Name:       "Invalid",
				Expression: "this is not a valid expression",
			},
			input:          map[string]interface{}{},
			expectedResult: false,
			expectError:    true,
		},
		{
			name: "Missing input field",
			rule: Rule{
				Name:       "MissingField",
				Expression: "input.nonexistent > 10",
			},
			input:          map[string]interface{}{},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, iss := env.Compile(tt.rule.Expression)
			if iss.Err() != nil {
				assert.True(t, tt.expectError)
				return
			}

			program, err := env.Program(ast)
			assert.NoError(t, err)

			tt.rule.CompiledProgram = program

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

func TestRule_Evaluate_NilCompiledProgram(t *testing.T) {
	rule := Rule{
		Name:       "NilProgram",
		Expression: "true",
	}

	_, err := rule.Evaluate(map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compiled program is nil")
}
