package run

import (
	"context"
	"strings"
	"testing"

	"admin/server/internal/module/ai/chat"
)

func int64Pointer(value int64) *int64 { return &value }

func TestResolveContextLimitsUsesConfiguredWindow(t *testing.T) {
	window, output := resolveContextLimits(AgentConfig{
		ContextWindowTokens: int64Pointer(200000), MaxOutputTokens: int64Pointer(8192),
	})
	if window != 200000 || output != 8192 {
		t.Fatalf("limits = %d/%d", window, output)
	}
	window, output = resolveContextLimits(AgentConfig{})
	if window != defaultContextWindowTokens || output != defaultMaxOutputTokens {
		t.Fatalf("default limits = %d/%d", window, output)
	}
	window, output = resolveContextLimits(AgentConfig{
		ContextWindowTokens: int64Pointer(4096), MaxOutputTokens: int64Pointer(4096),
	})
	if output >= window {
		t.Fatalf("output limit %d was not clamped below window %d", output, window)
	}
}

func TestEstimateTextTokensIsConservative(t *testing.T) {
	if got := estimateTextTokens(""); got != 0 {
		t.Fatalf("empty tokens = %d", got)
	}
	if got := estimateTextTokens("你好世界"); got < 4 {
		t.Fatalf("CJK tokens = %d", got)
	}
	if got := estimateTextTokens(strings.Repeat("a", 400)); got < 100 {
		t.Fatalf("ASCII tokens = %d", got)
	}
}

func TestStripZeroWidth(t *testing.T) {
	if got := stripZeroWidth("\u200B\u200C\u200D\uFEFF"); got != "" {
		t.Fatalf("zero-width text = %q", got)
	}
	if got := stripZeroWidth("思考\u200B过程"); got != "思考过程" {
		t.Fatalf("reasoning text = %q", got)
	}
}

func TestLatestUserQueryUsesNewestUserMessage(t *testing.T) {
	history := []chat.Message{
		{ID: 3, Role: chat.RoleUser, Content: "最新的问题"},
		{ID: 2, Role: chat.RoleAssistant, Content: "回答"},
		{ID: 1, Role: chat.RoleUser, Content: "旧问题"},
	}
	if got := latestUserQuery(history); got != "最新的问题" {
		t.Fatalf("latest query = %q", got)
	}
}

func TestAssembleContextKeepsCurrentMessageAndDropsOlderOverflow(t *testing.T) {
	current := strings.Repeat("最", 900)
	history := []chat.Message{
		{ID: 3, Role: chat.RoleUser, Content: current},
		{ID: 2, Role: chat.RoleAssistant, Content: strings.Repeat("旧", 3000)},
	}
	messages, plan, err := (&Service{}).assembleContext(
		context.Background(), Run{UserID: 1, PlatformID: 1}, AgentConfig{
			ContextWindowTokens: int64Pointer(4096), MaxOutputTokens: int64Pointer(1024),
		}, history, nil, "", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %d, plan = %+v", len(messages), plan)
	}
	if messages[0].Role != "user" || messages[0].Content != current {
		t.Fatalf("current message was not preserved: %+v", messages[0])
	}
	if plan.DroppedMessages == 0 || plan.IncludedMessages != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestAssembleContextKeepsCurrentMessageOverBudget(t *testing.T) {
	current := strings.Repeat("最", 3000)
	history := []chat.Message{{ID: 1, Role: chat.RoleUser, Content: current}}
	messages, plan, err := (&Service{}).assembleContext(
		context.Background(), Run{UserID: 1, PlatformID: 1}, AgentConfig{
			ContextWindowTokens: int64Pointer(2048), MaxOutputTokens: int64Pointer(512),
		}, history, nil, "", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || !plan.OverBudget {
		t.Fatalf("over-budget current message was not preserved: messages=%d plan=%+v", len(messages), plan)
	}
}
