package api

import (
	"fmt"
	"log/slog"

	"github.com/bufbuild/protovalidate-go"
	"github.com/nats-io/nats.go"
	"github.com/sandrolain/rules/engine"
	"github.com/sandrolain/rules/models" // Add this import
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
		slog.Error("Error unmarshalling SetPolicy request", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		slog.Error("Error validating SetPolicy request", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	policy := convertProtoToModelPolicy(req.Policy)
	err := h.ruleEngine.AddPolicy(policy)
	if err != nil {
		slog.Error("Error adding policy", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	resp := &SetPolicyResponse{Success: true}
	if err := h.replyWithProto(msg, resp); err != nil {
		slog.Error("Error sending response", "error", err)
	}
}

func (h *NatsHandler) handleListPolicies(msg *nats.Msg) {
	policies := h.ruleEngine.GetAllPolicies()
	resp := &ListPoliciesResponse{
		Policies: make([]*Policy, len(policies)),
	}
	for i, p := range policies {
		resp.Policies[i] = convertModelToProtoPolicy(p)
	}
	if err := h.replyWithProto(msg, resp); err != nil {
		slog.Error("Error sending response", "error", err)
	}
}

func (h *NatsHandler) handleGetPolicy(msg *nats.Msg) {
	var req GetPolicyRequest
	if err := proto.Unmarshal(msg.Data, &req); err != nil {
		slog.Error("Error unmarshalling GetPolicy request", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		slog.Error("Error validating GetPolicy request", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	policy, err := h.ruleEngine.GetPolicy(req.Id)
	if err != nil {
		slog.Error("Error retrieving policy", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	resp := &GetPolicyResponse{
		Policy: convertModelToProtoPolicy(policy),
	}
	if err := h.replyWithProto(msg, resp); err != nil {
		slog.Error("Error sending response", "error", err)
	}
}

func (h *NatsHandler) handleDeletePolicy(msg *nats.Msg) {
	var req DeletePolicyRequest
	if err := proto.Unmarshal(msg.Data, &req); err != nil {
		slog.Error("Error unmarshalling DeletePolicy request", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		slog.Error("Error validating DeletePolicy request", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	err := h.ruleEngine.DeletePolicy(req.Id)
	if err != nil {
		slog.Error("Error deleting policy", "error", err)
		if err := h.replyWithError(msg, err); err != nil {
			slog.Error("Error sending error response", "error", err)
		}
		return
	}

	resp := &DeletePolicyResponse{Success: true}
	if err := h.replyWithProto(msg, resp); err != nil {
		slog.Error("Error sending response", "error", err)
	}
}

func (h *NatsHandler) replyWithError(msg *nats.Msg, err error) error {
	errResp := &ErrorResponse{Error: err.Error()}
	return h.replyWithProto(msg, errResp)
}

func (h *NatsHandler) replyWithProto(msg *nats.Msg, resp proto.Message) error {
	if msg.Reply == "" {
		slog.Warn("The message has no reply subject")
		return nil
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("error serializing the response: %v", err)
	}
	return msg.Respond(data)
}

// Helper functions to convert between proto and model types
func convertProtoToModelPolicy(p *Policy) models.Policy {
	return models.Policy{
		ID:         p.Id,
		Name:       p.Name,
		Expression: p.Expression,
		Rules:      convertProtoToModelRules(p.Rules),
		Thresholds: convertProtoToModelThresholds(p.Thresholds),
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

func convertProtoToModelThresholds(protoThresholds []*Threshold) []models.Threshold {
	thresholds := make([]models.Threshold, len(protoThresholds))
	for i, t := range protoThresholds {
		thresholds[i] = models.Threshold{
			ID:    t.Id,
			Value: t.Value,
		}
	}
	return thresholds
}

func convertModelToProtoPolicy(p models.Policy) *Policy {
	return &Policy{
		Id:         p.ID,
		Name:       p.Name,
		Expression: p.Expression,
		Rules:      convertModelToProtoRules(p.Rules),
		Thresholds: convertModelToProtoThresholds(p.Thresholds),
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

func convertModelToProtoThresholds(modelThresholds []models.Threshold) []*Threshold {
	thresholds := make([]*Threshold, len(modelThresholds))
	for i, t := range modelThresholds {
		thresholds[i] = &Threshold{
			Id:    t.ID,
			Value: int64(t.Value), // Converted from int to int64
		}
	}
	return thresholds
}

type InputAck struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
