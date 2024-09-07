package models

import (
	"github.com/google/cel-go/cel"
)

type Rule struct {
	Name            string
	Expression      string
	CompiledProgram cel.Program
}
