package sms

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	smslog "admin/server/internal/module/message/sms/log"
	"admin/server/internal/module/message/sms/logVerification"
	"admin/server/internal/module/message/sms/rateLimitPolicy"
	"admin/server/internal/module/message/sms/recipientRule"
	"admin/server/internal/module/message/sms/template"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/phone"
)

// logStore is the narrow slice of the SMS log repository the sender needs.
type logStore interface {
	CreatePending(context.Context, *smslog.Model) error
	FindActiveChallenge(context.Context, int64, string) (smslog.Model, error)
	Finish(context.Context, int64, int64, string, string, string, int, string, string, int64, *time.Time, time.Time) error
}

type verificationStore interface {
	Create(context.Context, *logVerification.Model) error
}

type policyStore interface {
	Catalog(context.Context, int64) (rateLimitPolicy.Catalog, error)
}

type Service struct {
	stores Stores
	keys   *secretkey.KeyRing
	now    func() time.Time
}

func NewService(stores Stores, keys *secretkey.KeyRing) *Service {
	return &Service{stores: stores, keys: keys, now: time.Now}
}

// PageInit returns the four fixed scenes for the admin page bootstrap.
func (s *Service) PageInit(context.Context) (PageInitResult, error) {
	catalog := template.FixedCatalog()
	scenes := make([]SceneOption, 0, len(catalog))
	for _, fixed := range catalog {
		scenes = append(scenes, SceneOption{Scene: fixed.Scene, Name: fixed.Name, ParameterKeys: fixed.ParameterKeys})
	}
	return PageInitResult{Scenes: scenes}, nil
}

func (s *Service) facts(ctx context.Context) (RuntimeFacts, error) {
	if s.stores.Runtime == nil {
		return RuntimeFacts{}, fmt.Errorf("sms runtime cache is unavailable")
	}
	return s.stores.Runtime.Load(ctx, s.stores.runtimeLoader())
}

// VerifyCodeReady reports readiness for one scene from the cached facts.
func (s *Service) VerifyCodeReady(ctx context.Context, scene string) (VerifyCodeReadiness, error) {
	if err := validScene(scene); err != nil {
		return VerifyCodeReadiness{}, err
	}
	if s.stores.Runtime == nil {
		return VerifyCodeReadiness{}, fmt.Errorf("sms runtime cache is unavailable")
	}
	return s.stores.Runtime.LoadReadiness(ctx, scene, func(computeContext context.Context) (VerifyCodeReadiness, error) {
		facts, err := s.facts(computeContext)
		if err != nil {
			return VerifyCodeReadiness{}, err
		}
		return readinessOf(facts, scene)
	})
}

// PreparePhoneVerifyCode reserves the two shared windows once and returns the
// TTL and the resend delay. A deny from the rules or an exhausted window fails.
func (s *Service) PreparePhoneVerifyCode(ctx context.Context, input PhoneVerifyCodePrepareInput) (PhoneVerifyCodePreparation, error) {
	if err := validScene(input.Scene); err != nil {
		return PhoneVerifyCodePreparation{}, err
	}
	normalized, err := phone.Normalize(input.ToPhone)
	if err != nil {
		return PhoneVerifyCodePreparation{}, invalidRequest(fmt.Errorf("phone must be a mainland number"))
	}
	facts, err := s.facts(ctx)
	if err != nil {
		return PhoneVerifyCodePreparation{}, dependency(err)
	}
	readiness, err := readinessOf(facts, input.Scene)
	if err != nil {
		return PhoneVerifyCodePreparation{}, dependency(err)
	}
	if !readiness.Ready {
		return PhoneVerifyCodePreparation{}, dependency(ErrTemplateDisabled)
	}
	if s.keys == nil {
		return PhoneVerifyCodePreparation{}, dependency(fmt.Errorf("sms keys are unavailable"))
	}
	decision, err := evaluateRules(s.keys, facts.Rules, normalized)
	if err != nil {
		return PhoneVerifyCodePreparation{}, dependency(err)
	}
	if !decision.Allowed {
		return PhoneVerifyCodePreparation{}, forbidden(ErrRecipientDenied)
	}

	retry, err := s.reserve(ctx, input.PlatformID, normalized)
	if err != nil {
		return PhoneVerifyCodePreparation{}, err
	}
	return PhoneVerifyCodePreparation{TTLMinutes: readiness.TTLMinutes, ResendAfterSeconds: retry}, nil
}

// SendPreparedPhoneVerifyCode sends without counting the quota again. A code
// that was not delivered never reaches the Auth consumable store.
func (s *Service) SendPreparedPhoneVerifyCode(ctx context.Context, input PhoneVerifyCodeInput) (PhoneVerifyCodeResult, error) {
	if err := validScene(input.Scene); err != nil {
		return PhoneVerifyCodeResult{}, err
	}
	normalized, err := phone.Normalize(input.ToPhone)
	if err != nil {
		return PhoneVerifyCodeResult{}, invalidRequest(fmt.Errorf("phone must be a mainland number"))
	}
	now := s.now().UTC()
	if err := validatePreparedPhoneCodeInput(now, input); err != nil {
		return PhoneVerifyCodeResult{}, invalidRequest(err)
	}

	facts, err := s.facts(ctx)
	if err != nil {
		return PhoneVerifyCodeResult{}, dependency(err)
	}
	readiness, err := readinessOf(facts, input.Scene)
	if err != nil {
		return PhoneVerifyCodeResult{}, dependency(err)
	}
	if !readiness.Ready {
		return PhoneVerifyCodeResult{}, dependency(ErrTemplateDisabled)
	}
	if input.Preparation.TTLMinutes != readiness.TTLMinutes {
		return PhoneVerifyCodeResult{}, invalidRequest(fmt.Errorf("prepared ttl no longer matches sms readiness"))
	}
	if s.keys == nil || s.stores.Log == nil || s.stores.Verification == nil || s.stores.Sender == nil {
		return PhoneVerifyCodeResult{}, dependency(fmt.Errorf("sms sending dependencies are unavailable"))
	}

	tpl := facts.Templates[input.Scene]
	templateID, err := strconv.ParseInt(tpl.TencentTemplateID, 10, 64)
	if err != nil {
		return PhoneVerifyCodeResult{}, dependency(fmt.Errorf("sms template id is invalid"))
	}
	started := s.now().UTC()
	logRow, err := s.createPending(ctx, input, normalized, facts, templateID, started)
	if err != nil {
		return PhoneVerifyCodeResult{}, err
	}
	secretID, secretKey, err := decryptRuntimeCredentials(s.keys, facts.Config)
	if err != nil {
		s.finishBestEffort(ctx, logRow, smslog.StatusFailed, "", "", 0, "sms_credentials_unavailable", "sms credentials are unavailable", started)
		return PhoneVerifyCodeResult{}, dependency(fmt.Errorf("decrypt sms runtime credentials: %w", err))
	}

	result, err := s.stores.Sender.Send(ctx, SendInput{
		Region: facts.Config.Region, Endpoint: facts.Config.Endpoint,
		SecretID: secretID, SecretKey: secretKey,
		SDKAppID: facts.Config.SDKAppID, SignName: facts.Config.SignName,
		TemplateID: tpl.TencentTemplateID, ToPhone: normalized,
		TemplateParams: []string{input.Code, strconv.Itoa(input.Preparation.TTLMinutes)},
	})
	if err != nil {
		s.finishBestEffort(ctx, logRow, smslog.StatusFailed, "", "", 0, providerErrorCode(err), providerErrorSummary(err), started)
		return PhoneVerifyCodeResult{}, dependency(err)
	}

	latency := time.Since(started).Milliseconds()
	sentAt := s.now().UTC()
	if err := s.stores.Log.Finish(ctx, logRow.ID, input.PlatformID, smslog.StatusSent, result.RequestID, result.SerialNo, int(result.Fee), "", "", latency, &sentAt, s.now().UTC()); err != nil {
		return PhoneVerifyCodeResult{}, dependency(fmt.Errorf("finish sms log: %w", err))
	}
	return PhoneVerifyCodeResult{LogID: logRow.ID, RequestID: result.RequestID, SerialNo: result.SerialNo}, nil
}

func validatePreparedPhoneCodeInput(now time.Time, input PhoneVerifyCodeInput) error {
	if len(input.Code) != 6 {
		return fmt.Errorf("code must contain six ASCII digits")
	}
	for _, character := range input.Code {
		if character < '0' || character > '9' {
			return fmt.Errorf("code must contain six ASCII digits")
		}
	}
	if input.ChallengeID == "" || len(input.ChallengeID) > 128 || strings.TrimSpace(input.ChallengeID) != input.ChallengeID || strings.IndexFunc(input.ChallengeID, unicode.IsControl) >= 0 {
		return fmt.Errorf("challenge is invalid")
	}
	if input.Preparation.TTLMinutes < 1 || input.Preparation.TTLMinutes > 60 || !input.ExpiresAt.After(now) || input.ExpiresAt.After(now.Add(time.Duration(input.Preparation.TTLMinutes)*time.Minute)) {
		return fmt.Errorf("prepared payload is invalid")
	}
	return nil
}

// SendAdminTest uses the selected scene's example variables and the same shared
// windows, and never writes a verification row that Auth could consume.
func (s *Service) SendAdminTest(ctx context.Context, input AdminTestInput) (AdminTestResult, error) {
	if err := validScene(input.Scene); err != nil {
		return AdminTestResult{}, err
	}
	normalized, err := phone.Normalize(input.ToPhone)
	if err != nil {
		return AdminTestResult{}, invalidRequest(fmt.Errorf("phone must be a mainland number"))
	}
	facts, err := s.facts(ctx)
	if err != nil {
		return AdminTestResult{}, dependency(err)
	}
	readiness, err := readinessOf(facts, input.Scene)
	if err != nil {
		return AdminTestResult{}, dependency(err)
	}
	if !readiness.Ready {
		return AdminTestResult{}, dependency(ErrTemplateDisabled)
	}
	if s.keys == nil {
		return AdminTestResult{}, dependency(fmt.Errorf("sms keys are unavailable"))
	}
	decision, err := evaluateRules(s.keys, facts.Rules, normalized)
	if err != nil {
		return AdminTestResult{}, dependency(err)
	}
	if !decision.Allowed {
		return AdminTestResult{}, forbidden(ErrRecipientDenied)
	}
	if _, err := s.reserve(ctx, input.PlatformID, normalized); err != nil {
		return AdminTestResult{}, err
	}
	if s.stores.Log == nil || s.stores.Sender == nil {
		return AdminTestResult{}, dependency(fmt.Errorf("sms sending dependencies are unavailable"))
	}

	tpl := facts.Templates[input.Scene]
	templateID, err := strconv.ParseInt(tpl.TencentTemplateID, 10, 64)
	if err != nil {
		return AdminTestResult{}, dependency(fmt.Errorf("sms template id is invalid"))
	}
	variables := make([]string, 0, len(tpl.ParameterKeys))
	for _, key := range tpl.ParameterKeys {
		variables = append(variables, tpl.ExampleVariables[key])
	}
	started := s.now().UTC()
	phoneHMAC := recipientRule.HMACValue(s.keys, normalized)
	ciphertext, _, err := secretkey.EncryptSMSValue(s.keys.SMSEncryptionKey(), normalized)
	if err != nil {
		return AdminTestResult{}, dependency(err)
	}
	logRow := &smslog.Model{
		PlatformID: input.PlatformID, Scene: input.Scene, TemplateID: templateID,
		ToPhoneCiphertext: ciphertext, ToPhoneHint: phone.Hint(normalized), ToPhoneHMAC: phoneHMAC,
		Status: smslog.StatusPending, CreatedAt: started, UpdatedAt: started,
	}
	if err := s.stores.Log.CreatePending(ctx, logRow); err != nil {
		if errors.Is(err, smslog.ErrChallengeActive) {
			return AdminTestResult{}, conflict(err)
		}
		return AdminTestResult{}, dependency(fmt.Errorf("create sms log: %w", err))
	}
	secretID, secretKey, err := decryptRuntimeCredentials(s.keys, facts.Config)
	if err != nil {
		s.finishBestEffort(ctx, logRow, smslog.StatusFailed, "", "", 0, "sms_credentials_unavailable", "sms credentials are unavailable", started)
		return AdminTestResult{}, dependency(fmt.Errorf("decrypt sms runtime credentials: %w", err))
	}

	result, err := s.stores.Sender.Send(ctx, SendInput{
		Region: facts.Config.Region, Endpoint: facts.Config.Endpoint,
		SecretID: secretID, SecretKey: secretKey,
		SDKAppID: facts.Config.SDKAppID, SignName: facts.Config.SignName,
		TemplateID: tpl.TencentTemplateID, ToPhone: normalized, TemplateParams: variables,
	})
	if err != nil {
		s.finishBestEffort(ctx, logRow, smslog.StatusFailed, "", "", 0, providerErrorCode(err), providerErrorSummary(err), started)
		return AdminTestResult{}, dependency(err)
	}
	latency := time.Since(started).Milliseconds()
	sentAt := s.now().UTC()
	if err := s.stores.Log.Finish(ctx, logRow.ID, input.PlatformID, smslog.StatusSent, result.RequestID, result.SerialNo, int(result.Fee), "", "", latency, &sentAt, s.now().UTC()); err != nil {
		return AdminTestResult{}, dependency(fmt.Errorf("finish sms log: %w", err))
	}
	return AdminTestResult{LogID: logRow.ID, Status: smslog.StatusSent, RequestID: result.RequestID, SerialNo: result.SerialNo}, nil
}

func (s *Service) reserve(ctx context.Context, platformID int64, normalizedPhone string) (int, error) {
	if s.keys == nil || s.stores.Policy == nil || s.stores.Limiter == nil {
		return 0, dependency(fmt.Errorf("sms rate limit dependencies are unavailable"))
	}
	catalog, err := s.stores.Policy.Catalog(ctx, platformID)
	if err != nil {
		return 0, dependency(err)
	}
	phoneHMAC := recipientRule.HMACValue(s.keys, normalizedPhone)
	requests := make([]LimitRequest, 0, len(catalog.Policies))
	for _, policy := range catalog.Policies {
		requests = append(requests, LimitRequest{
			Key:    fmt.Sprintf("rate:sms:send:v1:%d:%s:%s", platformID, phoneHMAC, policy.Key),
			Limit:  policy.Limit,
			Window: time.Duration(policy.WindowSeconds) * time.Second,
		})
	}
	result, err := s.stores.Limiter.Reserve(ctx, requests...)
	if err != nil {
		return 0, dependency(err)
	}
	if !result.Allowed {
		return 0, rateLimited(ErrRateLimited)
	}
	return result.RetryAfterSeconds, nil
}

func (s *Service) createPending(ctx context.Context, input PhoneVerifyCodeInput, normalized string, facts RuntimeFacts, templateID int64, started time.Time) (*smslog.Model, error) {
	phoneHMAC := recipientRule.HMACValue(s.keys, normalized)
	ciphertext, _, err := secretkey.EncryptSMSValue(s.keys.SMSEncryptionKey(), normalized)
	if err != nil {
		return nil, dependency(err)
	}
	codeCiphertext, _, err := secretkey.EncryptSMSValue(s.keys.SMSEncryptionKey(), input.Code)
	if err != nil {
		return nil, dependency(err)
	}
	logRow := &smslog.Model{
		PlatformID: input.PlatformID, ChallengeID: &input.ChallengeID, UserID: input.UserID,
		Scene: input.Scene, TemplateID: templateID,
		ToPhoneCiphertext: ciphertext, ToPhoneHint: phone.Hint(normalized), ToPhoneHMAC: phoneHMAC,
		Status: smslog.StatusPending, CreatedAt: started, UpdatedAt: started,
	}
	if err := s.stores.Log.CreatePending(ctx, logRow); err != nil {
		if errors.Is(err, smslog.ErrChallengeActive) {
			return nil, conflict(err)
		}
		return nil, dependency(fmt.Errorf("create sms log: %w", err))
	}
	verification := &logVerification.Model{
		PlatformID: input.PlatformID, SMSLogID: logRow.ID, KeyVersion: "v1",
		CodeCiphertext: codeCiphertext, ExpiresAt: input.ExpiresAt, CreatedAt: started, UpdatedAt: started,
	}
	if err := s.stores.Verification.Create(ctx, verification); err != nil {
		s.finishBestEffort(ctx, logRow, smslog.StatusFailed, "", "", 0, "sms_verification_audit_failed", "sms verification audit could not be created", started)
		return nil, dependency(fmt.Errorf("create sms verification audit: %w", err))
	}
	return logRow, nil
}

// finishBestEffort closes a pending log as failed using a bounded detached
// context; the pending row is kept for audit when the finish itself fails.
func (s *Service) finishBestEffort(ctx context.Context, logRow *smslog.Model, status, requestID, serialNo string, fee int, code, summary string, started time.Time) {
	if logRow == nil || s.stores.Log == nil {
		return
	}
	cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	latency := time.Since(started).Milliseconds()
	_ = s.stores.Log.Finish(cleanupContext, logRow.ID, logRow.PlatformID, status, requestID, serialNo, fee, code, summary, latency, nil, time.Now().UTC())
}

func providerErrorCode(err error) string {
	var failure *ProviderError
	if errors.As(err, &failure) {
		return failure.Code
	}
	return "sms_request_failed"
}

func providerErrorSummary(err error) string {
	var failure *ProviderError
	if errors.As(err, &failure) {
		return failure.Summary
	}
	return "sms provider request failed"
}

func decryptRuntimeCredentials(keys *secretkey.KeyRing, configured ConfiguredConfig) (string, string, error) {
	if keys == nil {
		return "", "", fmt.Errorf("sms keys are unavailable")
	}
	secretID, err := secretkey.DecryptSMSValue(keys.SMSEncryptionKey(), configured.SecretIDCiphertext)
	if err != nil {
		return "", "", fmt.Errorf("decrypt sms secret id: %w", err)
	}
	secretKey, err := secretkey.DecryptSMSValue(keys.SMSEncryptionKey(), configured.SecretKeyCiphertext)
	if err != nil {
		return "", "", fmt.Errorf("decrypt sms secret key: %w", err)
	}
	return secretID, secretKey, nil
}
