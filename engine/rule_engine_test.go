package engine

import (
	"testing"

	"github.com/sandrolain/rules/cel"
	rcel "github.com/sandrolain/rules/cel"
	"github.com/sandrolain/rules/models"
	"github.com/stretchr/testify/assert"
)

func TestNewRuleEngine(t *testing.T) {
	re, err := NewRuleEngine()
	assert.NoError(t, err)
	assert.NotNil(t, re)
	assert.NotNil(t, re.policyEnv)
	assert.NotNil(t, re.ruleEnv)
	assert.NotNil(t, re.policies)
}

func TestCreatePolicyEnv(t *testing.T) {
	env, err := rcel.CreatePolicyEnv()
	assert.NoError(t, err)
	assert.NotNil(t, env)
}

func TestCreateRuleEnv(t *testing.T) {
	env, err := rcel.CreateRuleEnv()
	assert.NoError(t, err)
	assert.NotNil(t, env)
}

func TestRuleEngine_AddPolicy(t *testing.T) {
	re, _ := NewRuleEngine()

	tests := []struct {
		name        string
		policy      models.Policy
		expectError bool
	}{
		{
			name: "Valid policy",
			policy: models.Policy{
				ID:         "policy1",
				Name:       "ValidPolicy",
				Expression: "true",
				Rules: []models.Rule{
					{Name: "Rule1", Expression: "Result(10, false)"},
				},
			},
			expectError: false,
		},
		{
			name: "Policy with empty ID",
			policy: models.Policy{
				Name:       "EmptyIDPolicy",
				Expression: "true",
			},
			expectError: true,
		},
		{
			name: "Policy with invalid expression",
			policy: models.Policy{
				ID:         "policy2",
				Name:       "InvalidExpressionPolicy",
				Expression: "this is not a valid expression",
			},
			expectError: true,
		},
		{
			name: "Policy with invalid rule",
			policy: models.Policy{
				ID:         "policy3",
				Name:       "InvalidRulePolicy",
				Expression: "true",
				Rules: []models.Rule{
					{Name: "InvalidRule", Expression: "this is not a valid expression"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := re.AddPolicy(tt.policy)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				storedPolicy, err := re.GetPolicy(tt.policy.ID)
				assert.NoError(t, err)
				assert.Equal(t, tt.policy.Name, storedPolicy.Name)
				assert.Equal(t, tt.policy.Expression, storedPolicy.Expression)
				assert.Len(t, storedPolicy.Rules, len(tt.policy.Rules))
			}
		})
	}
}

func TestRuleEngine_GetPolicy(t *testing.T) {
	re, _ := NewRuleEngine()
	policy := models.Policy{
		ID:   "test_policy",
		Name: "TestPolicy",
	}
	re.AddPolicy(policy)

	t.Run("Existing policy", func(t *testing.T) {
		storedPolicy, err := re.GetPolicy("test_policy")
		assert.NoError(t, err)
		assert.Equal(t, policy.Name, storedPolicy.Name)
	})

	t.Run("Non-existing policy", func(t *testing.T) {
		_, err := re.GetPolicy("non_existing_policy")
		assert.Error(t, err)
	})
}

func TestRuleEngine_EvaluateRule(t *testing.T) {
	ruleEnv, _ := cel.CreateRuleEnv()

	tests := []struct {
		name           string
		rule           models.Rule
		input          map[string]interface{}
		expectedResult models.RuleResult
		expectError    bool
	}{
		{
			name:           "Simple true rule",
			rule:           models.Rule{Name: "TrueRule", Expression: "Result(10, false)"},
			input:          map[string]interface{}{},
			expectedResult: models.RuleResult{Score: 10, Stop: false, Passed: true, Executed: true},
			expectError:    false,
		},
		{
			name:           "Simple false rule",
			rule:           models.Rule{Name: "FalseRule", Expression: "Result(0, true)"},
			input:          map[string]interface{}{},
			expectedResult: models.RuleResult{Score: 0, Stop: true, Passed: true, Executed: true},
			expectError:    false,
		},
		{
			name:           "Input-dependent rule (true case)",
			rule:           models.Rule{Name: "InputRule", Expression: "Result(input.value, input.value > 10)"},
			input:          map[string]interface{}{"value": 15},
			expectedResult: models.RuleResult{Score: 15, Stop: true, Passed: true, Executed: true},
			expectError:    false,
		},
		{
			name:           "Input-dependent rule (false case)",
			rule:           models.Rule{Name: "InputRule", Expression: "Result(input.value, input.value > 10)"},
			input:          map[string]interface{}{"value": 5},
			expectedResult: models.RuleResult{Score: 5, Stop: false, Passed: true, Executed: true},
			expectError:    false,
		},
		{
			name:           "Invalid rule",
			rule:           models.Rule{Name: "InvalidRule", Expression: "this is not a valid expression"},
			input:          map[string]interface{}{},
			expectedResult: models.RuleResult{},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.BuildProgram(ruleEnv)
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

func TestRuleEngine_EvaluatePolicy(t *testing.T) {
	re, _ := NewRuleEngine()

	policy := models.Policy{
		ID:         "test_policy",
		Name:       "TestPolicy",
		Expression: "input.age >= 18",
		Rules: []models.Rule{
			{Name: "CountryRule", Expression: "Result(10, input.country in ['US', 'CA', 'UK'])"},
			{Name: "ScoreRule", Expression: "Result(input.score, input.score > 70)"},
		},
		Thresholds: []models.Threshold{
			{ID: "low", Value: 0},
			{ID: "medium", Value: 15},
			{ID: "high", Value: 25},
		},
	}
	re.AddPolicy(policy)

	tests := []struct {
		name              string
		input             map[string]interface{}
		expectedThreshold string
		expectedResults   []models.RuleResult
		expectError       bool
	}{
		{
			name:              "All rules stop",
			input:             map[string]interface{}{"age": 25, "country": "US", "score": 80},
			expectedThreshold: "low",
			expectedResults: []models.RuleResult{
				{Score: 10, Stop: true, Passed: true, Executed: true},
				{Score: 0, Stop: false, Passed: false, Executed: false},
			},
			expectError: false,
		},
		{
			name:              "Country rule not stop",
			input:             map[string]interface{}{"age": 25, "country": "FR", "score": 80},
			expectedThreshold: "high",
			expectedResults: []models.RuleResult{
				{Score: 10, Stop: false, Passed: true, Executed: true},
				{Score: 80, Stop: true, Passed: true, Executed: true},
			},
			expectError: false,
		},
		{
			name:              "Score rule fails",
			input:             map[string]interface{}{"age": 25, "country": "US", "score": 60},
			expectedThreshold: "low",
			expectedResults: []models.RuleResult{
				{Score: 10, Stop: true, Passed: true, Executed: true},
				{Score: 0, Stop: false, Passed: false, Executed: false},
			},
			expectError: false,
		},
		{
			name:            "Invalid input type",
			input:           map[string]interface{}{"age": "not a number", "country": "US", "score": "not a number"},
			expectedResults: nil,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threshold, results, err := re.EvaluatePolicy("test_policy", tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedThreshold, threshold)
				assert.Equal(t, tt.expectedResults, results)
			}
		})
	}

	t.Run("Non-existing policy", func(t *testing.T) {
		_, _, err := re.EvaluatePolicy("non_existing_policy", map[string]interface{}{})
		assert.Error(t, err)
	})
}

func TestRuleEngine_GetAllPolicies(t *testing.T) {
	re, _ := NewRuleEngine()

	policies := []models.Policy{
		{ID: "policy1", Name: "Policy1"},
		{ID: "policy2", Name: "Policy2"},
		{ID: "policy3", Name: "Policy3"},
	}

	for _, p := range policies {
		re.AddPolicy(p)
	}

	allPolicies := re.GetAllPolicies()
	assert.Len(t, allPolicies, len(policies))

	for _, p := range policies {
		found := false
		for _, ap := range allPolicies {
			if ap.ID == p.ID {
				found = true
				assert.Equal(t, p.Name, ap.Name)
				break
			}
		}
		assert.True(t, found, "Policy %s not found in GetAllPolicies result", p.ID)
	}
}

func TestRuleEngine_DeletePolicy(t *testing.T) {
	re, _ := NewRuleEngine()

	policy := models.Policy{ID: "test_policy", Name: "TestPolicy"}
	re.AddPolicy(policy)

	t.Run("Delete existing policy", func(t *testing.T) {
		err := re.DeletePolicy("test_policy")
		assert.NoError(t, err)

		_, err = re.GetPolicy("test_policy")
		assert.Error(t, err)
	})

	t.Run("Delete non-existing policy", func(t *testing.T) {
		err := re.DeletePolicy("non_existing_policy")
		assert.Error(t, err)
	})
}
