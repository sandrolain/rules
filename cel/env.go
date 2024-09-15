package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func CreatePolicyEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Declarations(
			decls.NewVar("input", decls.NewMapType(decls.String, decls.Any)),
		),
	)
}

func CreateRuleEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Declarations(
			decls.NewVar("input", decls.NewMapType(decls.String, decls.Any)),
		),
		cel.Function("Result",
			cel.Overload("Result_create",
				[]*cel.Type{cel.AnyType, cel.BoolType},
				cel.MapType(cel.StringType, cel.AnyType),
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					if len(args) != 2 {
						return types.NewErr("Result requires exactly two arguments")
					}
					var value interface{}
					switch v := args[0].(type) {
					case types.Int:
						value = int64(v)
					case types.Double:
						value = float64(v)
					default:
						return types.NewErr("The first argument must be an integer or a float")
					}
					boolVal, ok := args[1].(types.Bool)
					if !ok {
						return types.NewErr("The second argument must be a boolean")
					}
					return types.NewStringInterfaceMap(types.DefaultTypeAdapter, map[string]any{
						"value": value,
						"stop":  bool(boolVal),
					})
				}),
			),
		),
	)
}
