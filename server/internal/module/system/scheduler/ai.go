package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// AIManagementDefinitions lets the API describe and manage the same tasks as
// the Worker, without assembling provider or media executors in the API.
func AIManagementDefinitions() ([]TaskDefinition, error) {
	unavailable := func(context.Context, ExecutionContext) error {
		return errors.New("AI tasks execute only in the Worker")
	}
	return AITaskDefinitions(AITaskExecutors{unavailable, unavailable, unavailable, unavailable, unavailable, unavailable}, 5*time.Minute, 5*time.Minute)
}

// AITaskExecutors is the explicit worker wiring contract. The scheduler does
// not discover handlers by name; main supplies each business function.
type AITaskExecutors struct {
	RunExecute      TaskExecutor
	MediaPoll       TaskExecutor
	MediaImport     TaskExecutor
	UsageSettle     TaskExecutor
	RunRecover      TaskExecutor
	RunEventCleanup TaskExecutor
}

func AITaskDefinitions(executors AITaskExecutors, generationTimeout, transferTimeout time.Duration) ([]TaskDefinition, error) {
	const maxDuration = time.Duration(1<<63 - 1)
	if executors.RunExecute == nil || executors.MediaPoll == nil || executors.MediaImport == nil || executors.UsageSettle == nil || executors.RunRecover == nil || executors.RunEventCleanup == nil || generationTimeout <= 0 || transferTimeout <= 0 || transferTimeout > maxDuration-30*time.Second || generationTimeout > maxDuration-transferTimeout-30*time.Second {
		return nil, ErrInvalidTaskDefinition
	}
	empty := func(raw json.RawMessage) error {
		var value map[string]json.RawMessage
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return errors.New("AI task payload must be an object")
		}
		return nil
	}
	const mediaPollTimeout = 5*time.Minute + 30*time.Second
	return []TaskDefinition{
		{Type: "ai.run.execute", DisplayName: "AI运行执行", DisplayNameEnglish: "AI run execution", Queue: "default", Timeout: generationTimeout + transferTimeout + 30*time.Second, MaxAttempts: 1, ValidateParams: empty, Execute: executors.RunExecute},
		{Type: "ai.media.poll", DisplayName: "AI媒体轮询", DisplayNameEnglish: "AI media polling", Queue: "default", Timeout: mediaPollTimeout, MaxAttempts: 3, Backoff: publishBackoff, ValidateParams: empty, Execute: executors.MediaPoll},
		{Type: "ai.media.import", DisplayName: "AI媒体导入", DisplayNameEnglish: "AI media import", Queue: "default", Timeout: transferTimeout + 30*time.Second, MaxAttempts: 3, Backoff: publishBackoff, ValidateParams: empty, Execute: executors.MediaImport},
		{Type: "ai.usage.settle", DisplayName: "AI用量结算", DisplayNameEnglish: "AI usage settlement", Queue: "default", Timeout: 30 * time.Second, MaxAttempts: 10, Backoff: aiSettlementBackoff, ValidateParams: empty, Execute: executors.UsageSettle},
		{Type: "ai.run.recover", DisplayName: "AI运行恢复", DisplayNameEnglish: "AI run recovery", BuiltinKey: "ai.run.recover", Queue: "maintenance", Timeout: 20 * time.Second, MaxAttempts: 3, ValidateParams: empty, Execute: executors.RunRecover},
		{Type: "ai.run.event.cleanup", DisplayName: "AI运行事件清理", DisplayNameEnglish: "AI run event cleanup", BuiltinKey: "ai.run.event.cleanup", Queue: "maintenance", Timeout: 30 * time.Second, MaxAttempts: 3, ValidateParams: empty, Execute: executors.RunEventCleanup},
	}, nil
}

func aiSettlementBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 5 * time.Second
	for i := 1; i < attempt && delay < 300*time.Second; i++ {
		delay *= 2
		if delay > 300*time.Second {
			return 300 * time.Second
		}
	}
	return delay
}
