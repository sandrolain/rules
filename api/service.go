package api

import (
	"context"

	"github.com/sandrolain/rules/engine"
	"github.com/sandrolain/rules/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RulesServiceServerImpl struct {
	UnimplementedRulesServiceServer
	ruleEngine *engine.RuleEngine
}

func NewRulesServiceServer(ruleEngine *engine.RuleEngine) *RulesServiceServerImpl {
	return &RulesServiceServerImpl{
		ruleEngine: ruleEngine,
	}
}

func (s *RulesServiceServerImpl) SetPolicy(ctx context.Context, req *SetPolicyRequest) (*SetPolicyResponse, error) {
	policy := models.Policy{
		ID:         req.Policy.Id,
		Name:       req.Policy.Name,
		Expression: req.Policy.Expression,
		Rules:      make([]models.Rule, len(req.Policy.Rules)),
	}

	for i, rule := range req.Policy.Rules {
		policy.Rules[i] = models.Rule{
			Name:       rule.Name,
			Expression: rule.Expression,
		}
	}

	err := s.ruleEngine.AddPolicy(policy)
	if err != nil {
		return &SetPolicyResponse{Success: false, Error: err.Error()}, nil
	}

	return &SetPolicyResponse{Success: true}, nil
}

func (s *RulesServiceServerImpl) ListPolicies(ctx context.Context, req *ListPoliciesRequest) (*ListPoliciesResponse, error) {
	policies := s.ruleEngine.GetAllPolicies()
	response := &ListPoliciesResponse{
		Policies: make([]*Policy, len(policies)),
	}

	for i, policy := range policies {
		response.Policies[i] = &Policy{
			Name:       policy.Name,
			Expression: policy.Expression,
			Rules:      make([]*Rule, len(policy.Rules)),
		}
		for j, rule := range policy.Rules {
			response.Policies[i].Rules[j] = &Rule{
				Name:       rule.Name,
				Expression: rule.Expression,
			}
		}
	}

	return response, nil
}

func (s *RulesServiceServerImpl) GetPolicy(ctx context.Context, req *GetPolicyRequest) (*GetPolicyResponse, error) {
	policy, err := s.ruleEngine.GetPolicy(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "policy not found: %v", err)
	}

	response := &GetPolicyResponse{
		Policy: &Policy{
			Name:       policy.Name,
			Expression: policy.Expression,
			Rules:      make([]*Rule, len(policy.Rules)),
		},
	}

	for i, rule := range policy.Rules {
		response.Policy.Rules[i] = &Rule{
			Name:       rule.Name,
			Expression: rule.Expression,
		}
	}

	return response, nil
}

func (s *RulesServiceServerImpl) DeletePolicy(ctx context.Context, req *DeletePolicyRequest) (*DeletePolicyResponse, error) {
	err := s.ruleEngine.DeletePolicy(req.Id)
	if err != nil {
		return &DeletePolicyResponse{Success: false, Error: err.Error()}, nil
	}

	return &DeletePolicyResponse{Success: true}, nil
}
