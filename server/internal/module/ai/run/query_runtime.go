package run

import (
	"admin/server/internal/module/ai/conversation"
	pr "admin/server/internal/module/ai/providerRequest"
	"context"
	"encoding/json"
	"strconv"
	"time"
)

func (r *Runtime) ConversationAgent(ctx context.Context, id int64, at time.Time) (conversation.AgentRuntime, error) {
	s, e := r.Get(ctx, id, at)
	if e != nil {
		return conversation.AgentRuntime{}, e
	}
	var c RuntimeConfig
	if e = json.Unmarshal(s.Config, &c); e != nil {
		return conversation.AgentRuntime{}, e
	}
	for _, op := range c.Operations {
		if op == "text.generate" {
			if _, e := textUsageLines(s, 1, pr.TokenUsage{Known: true}); e != nil {
				return conversation.AgentRuntime{}, e
			}
		}
	}
	v := conversation.AgentRuntime{Capabilities: c.Capabilities, Operations: c.Operations, Parameters: c.Parameters, GenerationParams: c.GenerationParams, Limits: c.Limits, MultiplierPPM: s.MultiplierPPM, PriceVersionID: strconv.FormatInt(s.PriceVersionID, 10), Rates: make([]conversation.AgentRate, 0, len(c.Rates))}
	for _, r := range c.Rates {
		v.Rates = append(v.Rates, conversation.AgentRate{ID: strconv.FormatInt(r.ID, 10), Operation: r.Operation, UsageKey: r.UsageKey, Selector: r.Selector, Unit: r.Unit, UnitQuantity: r.UnitQuantity, BillingUnitPriceUnits: r.BillingUnitPriceUnits})
	}
	return v, nil
}
