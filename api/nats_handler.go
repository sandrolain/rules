package api

import (
	"fmt"

	"github.com/bufbuild/protovalidate-go"
	"github.com/nats-io/nats.go"
	"github.com/sandrolain/rules/engine"
	"github.com/sandrolain/rules/models" // Aggiungi questa importazione
	"google.golang.org/protobuf/proto"
)

const (
	SubjectPrefix = "rules.engine"
	SetPolicy     = SubjectPrefix + ".policy.set"
	ListPolicies  = SubjectPrefix + ".policy.list"
	GetPolicy     = SubjectPrefix + ".policy.get"
	DeletePolicy  = SubjectPrefix + ".policy.delete"
)

type NatsHandler struct {
	nc         *nats.Conn
	ruleEngine *engine.RuleEngine
	validator  *protovalidate.Validator
}

func NewNatsHandler(nc *nats.Conn, ruleEngine *engine.RuleEngine) (*NatsHandler, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize validator: %v", err)
	}

	return &NatsHandler{
		nc:         nc,
		ruleEngine: ruleEngine,
		validator:  validator,
	}, nil
}

func (h *NatsHandler) HandleRequests() error {
	if _, err := h.nc.Subscribe(SetPolicy, h.handleSetPolicy); err != nil {
		return err
	}
	if _, err := h.nc.Subscribe(ListPolicies, h.handleListPolicies); err != nil {
		return err
	}
	if _, err := h.nc.Subscribe(GetPolicy, h.handleGetPolicy); err != nil {
		return err
	}
	if _, err := h.nc.Subscribe(DeletePolicy, h.handleDeletePolicy); err != nil {
		return err
	}
	return nil
}

func (h *NatsHandler) handleSetPolicy(msg *nats.Msg) {
	var req SetPolicyRequest
	if err := proto.Unmarshal(msg.Data, &req); err != nil {
		h.replyWithError(msg, err)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		h.replyWithError(msg, err)
		return
	}

	policy := convertProtoToModelPolicy(req.Policy)
	err := h.ruleEngine.AddPolicy(policy)
	if err != nil {
		h.replyWithError(msg, err)
		return
	}

	resp := &SetPolicyResponse{Success: true}
	h.replyWithProto(msg, resp)
}

func (h *NatsHandler) handleListPolicies(msg *nats.Msg) {
	policies := h.ruleEngine.GetAllPolicies()
	resp := &ListPoliciesResponse{
		Policies: make([]*Policy, len(policies)),
	}
	for i, p := range policies {
		resp.Policies[i] = convertModelToProtoPolicy(p)
	}
	h.replyWithProto(msg, resp)
}

func (h *NatsHandler) handleGetPolicy(msg *nats.Msg) {
	var req GetPolicyRequest
	if err := proto.Unmarshal(msg.Data, &req); err != nil {
		h.replyWithError(msg, err)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		h.replyWithError(msg, err)
		return
	}

	policy, err := h.ruleEngine.GetPolicy(req.Id)
	if err != nil {
		h.replyWithError(msg, err)
		return
	}

	resp := &GetPolicyResponse{
		Policy: convertModelToProtoPolicy(policy),
	}
	h.replyWithProto(msg, resp)
}

func (h *NatsHandler) handleDeletePolicy(msg *nats.Msg) {
	var req DeletePolicyRequest
	if err := proto.Unmarshal(msg.Data, &req); err != nil {
		h.replyWithError(msg, err)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		h.replyWithError(msg, err)
		return
	}

	err := h.ruleEngine.DeletePolicy(req.Id)
	if err != nil {
		h.replyWithError(msg, err)
		return
	}

	resp := &DeletePolicyResponse{Success: true}
	h.replyWithProto(msg, resp)
}

func (h *NatsHandler) replyWithError(msg *nats.Msg, err error) {
	errResp := &ErrorResponse{Error: err.Error()}
	h.replyWithProto(msg, errResp)
}

func (h *NatsHandler) replyWithProto(msg *nats.Msg, resp proto.Message) {
	data, err := proto.Marshal(resp)
	if err != nil {
		fmt.Printf("Error marshaling response: %v\n", err)
		return
	}
	msg.Respond(data)
}

// Helper functions to convert between proto and model types
func convertProtoToModelPolicy(p *Policy) models.Policy {
	return models.Policy{
		ID:         p.Id,
		Name:       p.Name,
		Expression: p.Expression,
		Rules:      convertProtoToModelRules(p.Rules),
	}
}

func convertProtoToModelRules(protoRules []*Rule) []models.Rule {
	rules := make([]models.Rule, len(protoRules))
	for i, r := range protoRules {
		rules[i] = models.Rule{
			Name:       r.Name,
			Expression: r.Expression,
		}
	}
	return rules
}

func convertModelToProtoPolicy(p models.Policy) *Policy {
	return &Policy{
		Id:         p.ID,
		Name:       p.Name,
		Expression: p.Expression,
		Rules:      convertModelToProtoRules(p.Rules),
	}
}

func convertModelToProtoRules(modelRules []models.Rule) []*Rule {
	rules := make([]*Rule, len(modelRules))
	for i, r := range modelRules {
		rules[i] = &Rule{
			Name:       r.Name,
			Expression: r.Expression,
		}
	}
	return rules
}
