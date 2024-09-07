package utils

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/stretchr/testify/assert"
)

func TestBuildExpression(t *testing.T) {
	env, err := cel.NewEnv(cel.Declarations(
		decls.NewVar("input", decls.NewMapType(decls.String, decls.Any)),
	))
	assert.NoError(t, err)

	tests := []struct {
		name        string
		expression  string
		expectError bool
	}{
		{
			name:        "Valid simple expression",
			expression:  "input.age > 18",
			expectError: false,
		},
		{
			name:        "Valid complex expression",
			expression:  "input.age >= 18 && input.country in ['US', 'CA', 'UK']",
			expectError: false,
		},
		{
			name:        "Invalid expression - syntax error",
			expression:  "this is not a valid expression",
			expectError: true,
		},
		{
			name:        "Invalid expression - undefined variable",
			expression:  "undefinedVar > 10",
			expectError: true,
		},
		{
			name:        "Valid expression with multiple conditions",
			expression:  "input.age > 18 && input.score >= 75 || input.vip == true",
			expectError: false,
		},
		{
			name:        "Valid expression with string operations",
			expression:  "input.name.startsWith('A') && input.name.endsWith('Z')",
			expectError: false,
		},
		{
			name:        "Valid expression with list operations",
			expression:  "size(input.items) > 0 && 'apple' in input.items",
			expectError: false,
		},
		{
			name:        "Empty expression",
			expression:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := BuildExpression(env, tt.expression, tt.name)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, program)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, program)
			}
		})
	}
}

func TestBuildExpression_WithDifferentEnvs(t *testing.T) {
	tests := []struct {
		name        string
		envSetup    func() *cel.Env
		expression  string
		expectError bool
	}{
		{
			name: "Environment with custom function",
			envSetup: func() *cel.Env {
				env, _ := cel.NewEnv(
					cel.Function("customFunc",
						cel.Overload("customFunc_string_int",
							[]*cel.Type{cel.StringType, cel.IntType},
							cel.BoolType,
							cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
								s := lhs.(types.String)
								i := rhs.(types.Int)
								return types.Bool(s.Size().(types.Int) > i)
							}),
						),
					),
				)
				return env
			},
			expression:  "customFunc('test', 3)",
			expectError: false,
		},
		{
			name: "Environment with type",
			envSetup: func() *cel.Env {
				env, err := cel.NewEnv(
					cel.Declarations(
						decls.NewVar("Name", decls.String),
						decls.NewVar("Age", decls.Int),
					),
				)
				if err != nil {
					panic(err)
				}
				return env
			},
			expression:  "Name.startsWith('A') && Age > 18",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := tt.envSetup()
			program, err := BuildExpression(env, tt.expression, tt.name)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, program)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, program)
			}
		})
	}
}
