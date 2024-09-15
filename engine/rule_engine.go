package engine

import (
	"fmt"
	"sort"

	rcel "github.com/sandrolain/rules/cel"
	"github.com/sandrolain/rules/models"
	"github.com/sandrolain/rules/utils"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type RuleEngine struct {
	policyEnv *cel.Env
	ruleEnv   *cel.Env
	policies  map[string]models.Policy
}

func NewRuleEngine() (*RuleEngine, error) {
	policyEnv, err := rcel.CreatePolicyEnv()
	if err != nil {
		return nil, fmt.Errorf("error creating policy CEL environment: %v", err)
	}

	ruleEnv, err := rcel.CreateRuleEnv()
	if err != nil {
		return nil, fmt.Errorf("error creating rule CEL environment: %v", err)
	}

	return &RuleEngine{
		policyEnv: policyEnv,
		ruleEnv:   ruleEnv,
		policies:  make(map[string]models.Policy),
	}, nil
}

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

func (re *RuleEngine) AddPolicy(policy models.Policy) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID cannot be empty")
	}
	if policy.Expression != "" {
		program, err := utils.BuildExpression(re.policyEnv, policy.Expression, policy.Name)
		if err != nil {
			return fmt.Errorf("error compiling policy expression: %v", err)
		}
		policy.CompiledProgram = program
	}

	// Compile all rules
	for i, rule := range policy.Rules {
		if rule.CompiledProgram == nil {
			program, err := utils.BuildExpression(re.ruleEnv, rule.Expression, rule.Name)
			if err != nil {
				return fmt.Errorf("error compiling rule %s: %v", rule.Name, err)
			}
			policy.Rules[i].CompiledProgram = program
		}
	}

	// Sort thresholds
	sort.Slice(policy.Thresholds, func(i, j int) bool {
		return policy.Thresholds[i].Value < policy.Thresholds[j].Value
	})

	re.policies[policy.ID] = policy
	return nil
}

func (re *RuleEngine) GetPolicy(id string) (models.Policy, error) {
	policy, exists := re.policies[id]
	if !exists {
		return models.Policy{}, fmt.Errorf("policy not found: %s", id)
	}
	return policy, nil
}

func (re *RuleEngine) EvaluatePolicy(policyID string, input map[string]interface{}) (string, []models.RuleResult, error) {
	policy, exists := re.policies[policyID]
	if !exists {
		return "", nil, fmt.Errorf("policy %s not found", policyID)
	}

	shouldExecute, err := policy.ShouldExecute(input)
	if err != nil {
		return "", nil, fmt.Errorf("error evaluating policy expression: %v", err)
	}
	if !shouldExecute {
		return "", nil, nil
	}

	return policy.Evaluate(input)
}

func (re *RuleEngine) GetAllPolicies() []models.Policy {
	policies := make([]models.Policy, 0, len(re.policies))
	for _, policy := range re.policies {
		policies = append(policies, policy)
	}
	return policies
}

// Add this method to the RuleEngine struct

func (re *RuleEngine) DeletePolicy(id string) error {
	if _, exists := re.policies[id]; !exists {
		return fmt.Errorf("policy not found: %s", id)
	}
	delete(re.policies, id)
	return nil
}
