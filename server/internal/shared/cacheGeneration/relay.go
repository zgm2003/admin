package cachegeneration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"regexp"
	"time"
)

const (
	relayClaimLimit = 50
	relayIdle       = time.Second
	relayMaxBackoff = 5 * time.Minute
)

type relayRepository interface {
	Current(context.Context, Scope) (int64, error)
	ClaimPending(context.Context, int, string, time.Time, time.Duration) ([]Event, error)
	MarkPublished(context.Context, int64, string, time.Time) error
	Reschedule(context.Context, int64, string, string, time.Time, time.Time) error
}

type relayStore interface {
	Reconcile(context.Context, Scope, int64) (PublishResult, error)
}

// Relay 把已提交的 outbox 事件发布为 Redis ready 状态。
// 单个事件失败不会终止循环；进程 context 取消时快速退出。
type Relay struct {
	repository relayRepository
	store      relayStore
	logger     *slog.Logger
	now        func() time.Time
}

func NewRelay(repository relayRepository, store relayStore, logger *slog.Logger) *Relay {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Relay{repository: repository, store: store, logger: logger, now: time.Now}
}

// Run 按 1 秒空闲轮询运行；context 取消返回 nil，只有不可恢复的装配错误才返回 error。
func (r *Relay) Run(ctx context.Context) error {
	if r == nil || r.repository == nil || r.store == nil {
		return fmt.Errorf("cache generation relay is not configured")
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if _, err := r.RunOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			r.logger.Error("cache generation relay iteration failed", "errorClass", ErrorClass(err))
		}
		timer := time.NewTimer(relayIdle)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

// RunOnce 处理一轮 claim，返回本轮成功发布的事件数。
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	if r == nil || r.repository == nil || r.store == nil {
		return 0, fmt.Errorf("cache generation relay is not configured")
	}
	now := r.now().UTC()
	token, err := newMutationToken()
	if err != nil {
		return 0, err
	}
	events, err := r.repository.ClaimPending(ctx, relayClaimLimit, token, now, MutationLeaseTTL)
	if err != nil {
		return 0, fmt.Errorf("claim cache generation outbox: %w", err)
	}
	published := 0
	for _, event := range events {
		if ctx.Err() != nil {
			return published, nil
		}
		if err := r.publish(ctx, event, token, now); err != nil {
			r.logger.Error("cache generation outbox event remains pending",
				"namespace", event.Scope.Namespace, "scopeKey", event.Scope.ScopeKey,
				"generation", event.Generation, "outboxId", event.ID,
				"attempts", event.Attempts, "errorClass", ErrorClass(err))
			continue
		}
		published++
	}
	return published, nil
}

func (r *Relay) publish(ctx context.Context, event Event, token string, now time.Time) error {
	authoritative, err := r.repository.Current(ctx, event.Scope)
	if err != nil {
		return r.reschedule(ctx, event, token, err, now)
	}
	// 旧事件只发布最新权威 generation，绝不降级。
	if _, err := r.store.Reconcile(ctx, event.Scope, authoritative); err != nil {
		return r.reschedule(ctx, event, token, err, now)
	}
	if err := r.repository.MarkPublished(ctx, event.ID, token, now); err != nil {
		if errors.Is(err, ErrOutboxClaimLost) {
			r.logger.Warn("cache generation outbox claim was taken over",
				"outboxId", event.ID, "generation", event.Generation)
			return nil
		}
		return err
	}
	return nil
}

func (r *Relay) reschedule(ctx context.Context, event Event, token string, cause error, now time.Time) error {
	availableAt := now.Add(backoffDelay(event.Attempts))
	if err := r.repository.Reschedule(ctx, event.ID, token, safeOutboxError(cause), availableAt, now); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// backoffDelay 为 min(1s * 2^(attempts-1), 5m) 并加入 0-20% jitter。
func backoffDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := time.Second
	for step := 1; step < attempts && delay < relayMaxBackoff; step++ {
		delay *= 2
		if delay > relayMaxBackoff {
			delay = relayMaxBackoff
		}
	}
	jitterRange := int64(delay / 5)
	if jitterRange < 1 {
		jitterRange = 1
	}
	return delay + time.Duration(rand.Int63n(jitterRange))
}

// ErrorClass 把错误映射为可安全记录的类别，不暴露连接串、token 或 payload。
func ErrorClass(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrOutboxClaimLost):
		return "claim-lost"
	case errors.Is(err, ErrStateCorrupt):
		return "state-corrupt"
	case errors.Is(err, ErrStateMissing):
		return "state-missing"
	case errors.Is(err, ErrUpdating):
		return "state-invalidating"
	case errors.Is(err, ErrGenerationChanged):
		return "generation-changed"
	case errors.Is(err, ErrGenerationRowMissing):
		return "generation-missing"
	default:
		return "dependency-unavailable"
	}
}

var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S+`),
	regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|secret|dsn|key)\s*=\s*\S+`),
}

// safeOutboxError 只保留错误类别与脱敏摘要；不写入连接串、token 或密钥。
func safeOutboxError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	for _, pattern := range redactPatterns {
		message = pattern.ReplaceAllString(message, "[redacted]")
	}
	return truncateOutboxError(ErrorClass(err) + ": " + message)
}
