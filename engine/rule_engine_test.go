package engine

import (
	"testing"

	"github.com/sandrolain/rules/models"
	"github.com/stretchr/testify/assert"
)

func TestRuleEngine_AddPolicy(t *testing.T) {
	re, err := NewRuleEngine()
	assert.NoError(t, err)

	tests := []struct {
		name        string
		policy      models.Policy
		expectError bool
	}{
		{
			name: "Add valid policy",
			policy: models.Policy{
				ID:         "policy1",
				Name:       "ValidPolicy",
				Expression: "true",
				Rules: []models.Rule{
					{Name: "Rule1", Expression: "input.value > 10"},
				},
			},
			expectError: false,
		},
		{
			name: "Add policy with invalid expression",
			policy: models.Policy{
				Name:       "InvalidPolicy",
				Expression: "this is not a valid expression",
			},
			expectError: true,
		},
		{
			name: "Add policy with invalid rule",
			policy: models.Policy{
				Name:       "PolicyWithInvalidRule",
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
				policy, err := re.GetPolicy(tt.policy.ID)
				assert.NoError(t, err)
				assert.Equal(t, tt.policy.Name, policy.Name)
				assert.Equal(t, tt.policy.Expression, policy.Expression)
				assert.Len(t, policy.Rules, len(tt.policy.Rules))
			}
		})
	}
}

func TestRuleEngine_EvaluatePolicy(t *testing.T) {
	re, err := NewRuleEngine()
	assert.NoError(t, err)

	policy := models.Policy{
		ID:         "test_policy",
		Name:       "TestPolicy",
		Expression: "input.age >= 18",
		Rules: []models.Rule{
			{Name: "Rule1", Expression: "input.country in ['US', 'CA', 'UK']"},
			{Name: "Rule2", Expression: "input.score > 70"},
		},
	}

	err = re.AddPolicy(policy)
	assert.NoError(t, err)

	tests := []struct {
		name           string
		input          map[string]interface{}
		expectedResult bool
		expectError    bool
	}{
		{
			name:           "Valid input, all rules pass",
			input:          map[string]interface{}{"age": 25, "country": "US", "score": 80},
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "Invalid age",
			input:          map[string]interface{}{"age": 16, "country": "IT", "score": 80},
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "Invalid country",
			input:          map[string]interface{}{"age": 25, "country": "FR", "score": 80},
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "Invalid score",
			input:          map[string]interface{}{"age": 25, "country": "US", "score": 60},
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "Invalid input type",
			input:          map[string]interface{}{"age": "not a number", "country": "US", "score": "not a number"},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := re.EvaluatePolicy(policy.ID, tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
