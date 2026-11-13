package chat

import "testing"

func TestMessageValueProjectsSettledUsage(t *testing.T) {
	runID, amount := int64(9), int64(1_816_000)
	input, cached, output := int64(4400), int64(128), int64(44)
	message := Message{
		ID: 18, Seq: 4, Role: RoleAssistant, Content: "answer", Status: MessageCompleted,
		RunID: &runID, UsageAmountUnits: &amount,
		UsageInputTokens: &input, UsageCachedInputTokens: &cached, UsageOutputTokens: &output,
	}
	value := messageValue(message)
	if value.Usage == nil {
		t.Fatal("expected usage block")
	}
	if value.Usage.Amount != "0.01816" || value.Usage.InputTokens != 4400 ||
		value.Usage.CachedInputTokens != 128 || value.Usage.OutputTokens != 44 {
		t.Fatalf("usage = %+v", value.Usage)
	}
}

func TestMessageValueOmitsUsageForUserAndUnsettledMessages(t *testing.T) {
	runID := int64(9)
	for _, message := range []Message{
		{ID: 17, Role: RoleUser, Status: MessageCompleted},
		{ID: 18, Role: RoleAssistant, Status: MessageStreaming, RunID: &runID},
		{ID: 19, Role: RoleAssistant, Status: MessageCompleted, RunID: &runID},
	} {
		if value := messageValue(message); value.Usage != nil {
			t.Fatalf("unexpected usage for message %+v: %+v", message, value.Usage)
		}
	}
}
