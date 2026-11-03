package run

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/ai/tool"
	"github.com/google/uuid"
)

type textClient interface {
	Generate(context.Context, pr.TextRequest) (pr.TextResponse, error)
}
type frozenCredentialReader interface {
	Load(context.Context, RuntimeSnapshot) (pr.FrozenConfig, error)
}
type toolVersionReader interface {
	Version(context.Context, int64) (tool.Version, error)
}
type toolExecutor interface {
	Execute(context.Context, tool.Version, json.RawMessage) (json.RawMessage, error)
}

type TextEngine struct {
	repository      *Repository
	credentials     frozenCredentialReader
	tools           toolVersionReader
	executor        toolExecutor
	responses, chat textClient
	now             func() time.Time
	files           textFileReader
	pressure        StreamPressure
}

func NewTextEngine(repository *Repository, credentials frozenCredentialReader, tools toolVersionReader, executor toolExecutor, responses, chat textClient) *TextEngine {
	return &TextEngine{repository: repository, credentials: credentials, tools: tools, executor: executor, responses: responses, chat: chat, now: time.Now}
}

func (e *TextEngine) Execute(ctx context.Context, input Model) error {
	if e == nil || e.repository == nil || e.credentials == nil {
		return ErrRuntimeUnavailable
	}
	row, err := e.repository.Find(ctx, input.ID)
	if err != nil {
		return err
	}
	if row.ExecutionStatus >= 3 {
		return nil
	}
	// Claim only the queued state. A redelivered or expired sending job must go
	// through recovery and must never repeat model/tool side effects.
	if row.ExecutionStatus != 0 {
		return nil
	}
	token := uuid.NewString()
	row, err = e.repository.Claim(ctx, row.ID, token, e.now().UTC())
	if err != nil {
		return err
	}
	if row.ExecutionStatus >= 3 {
		return nil
	}
	work, deadlineCancel := context.WithDeadline(ctx, row.DeadlineAt)
	defer deadlineCancel()
	work, leaseCancel, _ := LeaseHeartbeat(work, e.repository, row.ID, token, e.now)
	defer leaseCancel(nil)
	text, code := e.generate(work, row, token)
	status := int16(3)
	if code != "" {
		status = 4
	}
	// Bounded finalization is derived from the original scheduler context. Usage
	// must not be lost merely because the request deadline or cancel fired.
	finalize, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	return e.repository.FinishText(finalize, row.ID, token, status, code, text, e.now().UTC())
}

type historyTurn struct {
	Role    string              `json:"role"`
	Text    string              `json:"text"`
	Content json.RawMessage     `json:"content,omitempty"`
	Files   []InputFileSnapshot `json:"-"`
}
type executionInput struct {
	RunInput
	History []historyTurn `json:"history,omitempty"`
}

func (e *TextEngine) generate(ctx context.Context, row Model, token string) (string, string) {
	var snap RuntimeSnapshot
	var config RuntimeConfig
	var policy Policy
	var input executionInput
	if json.Unmarshal(row.ConfigSnapshot, &snap) != nil || json.Unmarshal(snap.Config, &config) != nil || json.Unmarshal(row.PolicySnapshot, &policy) != nil || policy.Validate() != nil || json.Unmarshal(row.InputSnapshot, &input) != nil {
		return "", "INVALID_SNAPSHOT"
	}
	if row.Operation != "text.generate" {
		return "", "UNSUPPORTED_OPERATION"
	}
	if e.pressure == nil {
		return "", "STREAM_PRESSURE_UNAVAILABLE"
	}
	credential, err := e.credentials.Load(ctx, snap)
	if err != nil {
		return "", "CREDENTIAL_UNAVAILABLE"
	}
	client := e.responses
	if snap.Protocol == 2 {
		client = e.chat
	}
	if client == nil || snap.Protocol < 1 || snap.Protocol > 2 {
		return "", "INVALID_PROTOCOL"
	}
	var limits struct{ MaxInputTokens, MaxOutputTokens int64 }
	if json.Unmarshal(config.Limits, &limits) != nil || limits.MaxInputTokens < 1 || limits.MaxOutputTokens < 1 {
		return "", "INVALID_LIMITS"
	}
	output := min(policy.MaxOutputTokens, limits.MaxOutputTokens)
	if input.MaxOutputTokens != nil {
		output = *input.MaxOutputTokens
	}
	if output < 1 || output > limits.MaxOutputTokens || output > policy.MaxOutputTokens {
		return "", "INVALID_LIMITS"
	}
	params, err := mergedParams(config.GenerationParams, config.Parameters)
	if err != nil {
		return "", "INVALID_PARAMETERS"
	}
	history, err := e.repository.TextHistory(ctx, row, policy.MaxTextInputBytes)
	if err != nil {
		return "", "CONTEXT_HISTORY_UNAVAILABLE"
	}
	native, err := e.prepareTextInput(ctx, row, snap.Protocol, history, input.RunInput, policy)
	if err != nil {
		return "", "CONTEXT_INPUT_UNAVAILABLE"
	}
	definitions := make([]pr.FunctionTool, 0, len(snap.Tools))
	bound := make(map[string]ToolSnapshot, len(snap.Tools))
	for _, t := range snap.Tools {
		var v struct {
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		}
		if json.Unmarshal(t.Config, &v) != nil || !json.Valid(v.InputSchema) {
			return "", "INVALID_TOOL_SNAPSHOT"
		}
		definitions = append(definitions, pr.FunctionTool{Name: t.Name, Description: v.Description, Parameters: v.InputSchema, Strict: false})
		bound[t.Name] = t
	}
	calls := int64(0)
	seenCalls := map[string]bool{}
	var accumulated strings.Builder
	deltas := newTextDeltaBatch(ctx, func(writeCtx context.Context, text string) error {
		return e.repository.AppendTextDelta(writeCtx, row.ID, token, text, e.now().UTC())
	}, e.pressure)
	for round := int64(1); round <= policy.MaxModelRounds; round++ {
		if _, pressureErr := deltas.interval(); pressureErr != nil {
			return accumulated.String(), "STREAM_PRESSURE_UNAVAILABLE"
		}
		if err = e.repository.Fence(ctx, row.ID, token, e.now().UTC()); err != nil {
			return accumulated.String(), "CANCELED_OR_LEASE_LOST"
		}
		contextBytes, countErr := nativeContextBytes(native)
		if countErr != nil || contextBytes+int64(len(snap.Prompt)) > policy.MaxTextInputBytes || contextBytes+int64(len(snap.Prompt)) > limits.MaxInputTokens {
			return accumulated.String(), "CONTEXT_LIMIT"
		}
		request := pr.TextRequest{Config: credential, Prompt: snap.Prompt, Input: native, Tools: definitions, MaxOutputTokens: output, Params: params}
		// Hash protocol input rather than persisting prompts or inline image bytes.
		summary, err := textRequestSummary(native)
		if err != nil {
			return accumulated.String(), "INVALID_REQUEST"
		}
		fact, err := e.repository.StartRequest(ctx, row.ID, token, int(round), summary, e.now().UTC())
		if err != nil {
			return accumulated.String(), "REQUEST_INTENT_FAILED"
		}
		request.OnTextDelta = deltas.Append
		response, callErr := client.Generate(ctx, request)
		if flushErr := deltas.Flush(); flushErr != nil {
			callErr = errors.Join(callErr, flushErr)
		}
		save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		err = e.repository.FinishRequest(save, row.ID, token, fact.ID, response, callErr, e.now().UTC())
		cancel()
		if err != nil {
			return accumulated.String(), "REQUEST_RESULT_FAILED"
		}
		accumulated.WriteString(response.Text)
		if callErr != nil {
			return accumulated.String(), "PROVIDER_REQUEST_FAILED"
		}
		if !response.Usage.Known {
			return accumulated.String(), "USAGE_UNKNOWN"
		}
		if len(response.Calls) == 0 {
			return accumulated.String(), ""
		}
		if round == policy.MaxModelRounds {
			return accumulated.String(), "MODEL_ROUND_LIMIT"
		}
		native = append(native, response.OutputItems...)
		for _, call := range response.Calls {
			calls++
			if calls > policy.MaxToolCalls {
				return accumulated.String(), "TOOL_CALL_LIMIT"
			}
			binding, ok := bound[call.Name]
			if !ok || seenCalls[call.CallID] || e.tools == nil || e.executor == nil {
				return accumulated.String(), "TOOL_CALL_INVALID"
			}
			seenCalls[call.CallID] = true
			version, err := e.tools.Version(ctx, binding.VersionID)
			if err != nil || version.ToolID != binding.ToolID {
				return accumulated.String(), "TOOL_VERSION_UNAVAILABLE"
			}
			if err = e.repository.Fence(ctx, row.ID, token, e.now().UTC()); err != nil {
				return accumulated.String(), "CANCELED_OR_LEASE_LOST"
			}
			toolFact, err := e.repository.StartTool(ctx, row.ID, token, fact.ID, binding, call, e.now().UTC())
			if err != nil {
				return accumulated.String(), "TOOL_INTENT_FAILED"
			}
			result, toolErr := e.executor.Execute(ctx, version, json.RawMessage(call.Arguments))
			unknown := toolErr != nil && version.HasSideEffect == 1
			if toolErr != nil {
				result = json.RawMessage(`{"error":"TOOL_EXECUTION_FAILED"}`)
			}
			save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			err = e.repository.FinishTool(save, row.ID, token, toolFact.ID, result, unknown, e.now().UTC())
			cancel()
			if err != nil {
				return accumulated.String(), "TOOL_RESULT_FAILED"
			}
			if unknown {
				return accumulated.String(), "TOOL_OUTCOME_UNKNOWN"
			}
			native = append(native, nativeToolResult(snap.Protocol, call.CallID, result))
		}
	}
	return accumulated.String(), "MODEL_ROUND_LIMIT"
}

func mergedParams(values ...json.RawMessage) (json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for _, v := range values {
		if len(v) == 0 {
			continue
		}
		var data map[string]json.RawMessage
		if json.Unmarshal(v, &data) != nil || data == nil {
			return nil, ErrInvalidSnapshot
		}
		for k, val := range data {
			out[k] = val
		}
	}
	return json.Marshal(out)
}
func nativeTurn(protocol int16, turn historyTurn) json.RawMessage {
	var v any
	if protocol == 2 {
		v = map[string]string{"role": turn.Role, "content": turn.Text}
	} else {
		kind := "input_text"
		if turn.Role == "assistant" {
			kind = "output_text"
		}
		v = map[string]any{"role": turn.Role, "content": []map[string]string{{"type": kind, "text": turn.Text}}}
	}
	raw, _ := json.Marshal(v)
	return raw
}
func nativeToolResult(protocol int16, id string, result json.RawMessage) json.RawMessage {
	var v any
	if protocol == 2 {
		v = map[string]string{"role": "tool", "tool_call_id": id, "content": string(result)}
	} else {
		v = map[string]string{"type": "function_call_output", "call_id": id, "output": string(result)}
	}
	raw, _ := json.Marshal(v)
	return raw
}

var _ Executor = (*TextEngine)(nil)
var _ = errors.Is
