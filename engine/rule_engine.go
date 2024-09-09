package engine

import (
	"fmt"

	"github.com/sandrolain/rules/models"
	"github.com/sandrolain/rules/utils"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
)

type RuleEngine struct {
	env      *cel.Env
	policies map[string]models.Policy // Key is now the policy ID
}

func NewRuleEngine() (*RuleEngine, error) {
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("input", decls.NewMapType(decls.String, decls.Any)),
			decls.NewVar("policy", decls.NewMapType(decls.String, decls.Any)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating CEL environment: %v", err)
	}
	return &RuleEngine{
		env:      env,
		policies: make(map[string]models.Policy),
	}, nil
}

func (re *RuleEngine) AddPolicy(policy models.Policy) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID cannot be empty")
	}
	if policy.Expression != "" {
		program, err := utils.BuildExpression(re.env, policy.Expression, policy.Name)
		if err != nil {
			return fmt.Errorf("error compiling policy expression: %v", err)
		}
		policy.CompiledProgram = program
	}

	// Compile all rules
	for i, rule := range policy.Rules {
		if rule.CompiledProgram == nil {
			program, err := utils.BuildExpression(re.env, rule.Expression, rule.Name)
			if err != nil {
				return fmt.Errorf("error compiling rule %s: %v", rule.Name, err)
			}
			policy.Rules[i].CompiledProgram = program
		}
	}

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

func (re *RuleEngine) EvaluateRule(rule models.Rule, input map[string]interface{}) (bool, error) {
	return rule.Evaluate(input)
}

func (re *RuleEngine) EvaluatePolicy(policyID string, input map[string]interface{}) (bool, error) {
	policy, exists := re.policies[policyID]
	if !exists {
		return false, fmt.Errorf("policy %s not found", policyID)
	}

	for _, rule := range policy.Rules {
		result, err := re.EvaluateRule(rule, input)
		if err != nil {
			return false, fmt.Errorf("error evaluating rule %s: %v", rule.Name, err)
		}
		if !result {
			return false, nil // If a rule fails, the entire policy fails
		}
	}

	return true, nil // All rules have been satisfied
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
