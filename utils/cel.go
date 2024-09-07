package utils

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

// BuildExpression compiles a CEL expression and returns a Program
func BuildExpression(env *cel.Env, expression string, name string) (cel.Program, error) {
	ast, iss := env.Compile(expression)
	if iss.Err() != nil {
		return nil, fmt.Errorf("error compiling expression %s: %v", name, iss.Err())
	}

	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("error creating program for %s: %v", name, err)
	}

	return program, nil
}
