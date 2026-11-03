package conversation

import (
	"admin/server/internal/shared/apperror"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrAgentNotConsumable = errors.New("AI agent has no effective priced operation")

type AgentRate struct {
	ID                    string          `json:"id"`
	Operation             string          `json:"operation"`
	UsageKey              string          `json:"usageKey"`
	Selector              json.RawMessage `json:"selector"`
	Unit                  string          `json:"unit"`
	UnitQuantity          string          `json:"unitQuantity"`
	BillingUnitPriceUnits int64           `json:"billingUnitPriceUnits"`
}
type AgentRuntime struct {
	ParameterOptions []ParameterOption `json:"parameterOptions"`
	Capabilities     []string          `json:"capabilities"`
	Operations       []string          `json:"operations"`
	Parameters       json.RawMessage   `json:"parameters"`
	GenerationParams json.RawMessage   `json:"generationParams"`
	Limits           json.RawMessage   `json:"limits"`
	MultiplierPPM    int64             `json:"multiplierPpm"`
	PriceVersionID   string            `json:"priceVersionId"`
	Rates            []AgentRate       `json:"rates"`
}
type ParameterOption struct {
	Operation string            `json:"operation"`
	Key       string            `json:"key"`
	Label     string            `json:"label"`
	Options   []ParameterChoice `json:"options"`
}
type ParameterChoice struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}
type AgentCandidate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	AgentRuntime
}
type CandidatePage struct {
	List         []AgentCandidate `json:"list"`
	HasMore      bool             `json:"hasMore"`
	NextBeforeID string           `json:"nextBeforeId"`
}
type AgentResolver func(context.Context, int64, time.Time) (AgentRuntime, error)

func (s *Service) SetAgentResolver(r AgentResolver) { s.agents = r }
func PublicAgentRuntime(v AgentRuntime) (AgentRuntime, error) {
	ops := make([]string, 0)
	caps := make([]string, 0)
	for _, op := range v.Operations {
		if op == "video.generate" {
			continue
		}
		covered := false
		for _, r := range v.Rates {
			if r.Operation == op {
				covered = true
			}
		}
		capability := strings.SplitN(op, ".", 2)[0]
		allowed := false
		for _, c := range v.Capabilities {
			if c == capability {
				allowed = true
			}
		}
		if covered && allowed {
			ops = append(ops, op)
			seen := false
			for _, c := range caps {
				if c == capability {
					seen = true
				}
			}
			if !seen {
				caps = append(caps, capability)
			}
		}
	}
	if len(ops) == 0 {
		return AgentRuntime{}, ErrAgentNotConsumable
	}

	v.ParameterOptions = make([]ParameterOption, 0)
	for _, op := range ops {
		choices := map[string]map[string]any{}
		for _, rate := range v.Rates {
			if rate.Operation != op {
				continue
			}
			var selector map[string]any
			if len(rate.Selector) == 0 {
				continue
			}
			if json.Unmarshal(rate.Selector, &selector) != nil {
				return AgentRuntime{}, errors.New("AI price selector invalid")
			}
			for _, key := range []string{"quality", "size", "seconds", "voice", "format", "count"} {
				if value, ok := selector[key]; ok {
					switch value.(type) {
					case string, float64:
						if choices[key] == nil {
							choices[key] = map[string]any{}
						}
						choices[key][fmt.Sprint(value)] = value
					}
				}
			}
		}
		for _, key := range []string{"quality", "size", "seconds", "voice", "format", "count"} {
			values := choices[key]
			if len(values) == 0 {
				continue
			}
			keys := make([]string, 0, len(values))
			for value := range values {
				keys = append(keys, value)
			}
			sort.Strings(keys)
			option := ParameterOption{Operation: op, Key: key, Label: key, Options: make([]ParameterChoice, 0, len(keys))}
			for _, value := range keys {
				option.Options = append(option.Options, ParameterChoice{Value: values[value], Label: value})
			}
			v.ParameterOptions = append(v.ParameterOptions, option)
		}
	}

	v.Operations = ops
	v.Capabilities = caps
	return v, nil
}
func (s *Service) Candidates(ctx context.Context, user, platform, before int64, limit int, keyword string) (CandidatePage, error) {
	out := CandidatePage{List: make([]AgentCandidate, 0)}
	if e := validOwner(user, platform); e != nil {
		return out, e
	}
	if before < 0 || limit < 1 || limit > 100 || len(keyword) > 200 {
		return out, apperror.InvalidRequest(nil)
	}
	if s.agents == nil {
		return out, apperror.DependencyUnavailable(nil)
	}
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	rows, e := s.repository.CandidateRows(ctx, before, limit, keyword)
	if e != nil {
		return out, mapError(e)
	}
	out.HasMore = len(rows) > limit
	if out.HasMore {
		rows = rows[:limit]
	}
	for _, row := range rows {
		out.NextBeforeID = strconv.FormatInt(row.ID, 10)
		runtime, e := s.agents(ctx, row.ID, s.now().UTC())
		if e != nil {
			return CandidatePage{}, mapError(e)
		}
		runtime, e = PublicAgentRuntime(runtime)
		if e != nil {
			if errors.Is(e, ErrAgentNotConsumable) {
				continue
			}
			return CandidatePage{}, mapError(e)
		}
		out.List = append(out.List, AgentCandidate{ID: strconv.FormatInt(row.ID, 10), Name: row.Name, AgentRuntime: runtime})
	}
	return out, nil
}

type ConversationResponse struct {
	ID                  string    `json:"id"`
	AgentID             string    `json:"agentId"`
	Title               string    `json:"title"`
	LastMessageSequence int64     `json:"lastMessageSequence"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func publicConversation(m Model) ConversationResponse {
	return ConversationResponse{strconv.FormatInt(m.ID, 10), strconv.FormatInt(m.AgentID, 10), m.Title, m.LastMessageSequence, m.CreatedAt, m.UpdatedAt}
}
