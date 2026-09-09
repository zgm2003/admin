package mail

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	maillog "admin/server/internal/module/message/mail/log"
	ratelimitpolicy "admin/server/internal/module/message/mail/rateLimitPolicy"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	mailtemplate "admin/server/internal/module/message/mail/template"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Service struct {
	stores         *Stores
	keys           *secretkey.KeyRing
	sender         Sender
	rules          RuleEvaluator
	limiter        Limiter
	policyStore    RateLimitPolicyStore
	readinessStore VerifyCodeReadinessStore
	runtime        *runtimeStore
}

func NewService(stores *Stores, keys *secretkey.KeyRing, sender Sender, rules RuleEvaluator, limiter Limiter, policyStore RateLimitPolicyStore) *Service {
	return &Service{stores: stores, keys: keys, sender: sender, rules: rules, limiter: limiter, policyStore: policyStore}
}

func (s *Service) SetVerifyCodeReadinessStore(store VerifyCodeReadinessStore) {
	s.readinessStore = store
}

func (s *Service) SetRuntimeStore(store *runtimeStore) { s.runtime = store }

func (s *Service) Send(ctx context.Context, in BusinessSendInput) (SendResult, error) {
	return s.send(ctx, in, SendModeBusiness)
}

// VerifyCodeReady reports whether the platform can send a verification email
// for the given scene, together with the channel's single TTL authority.
// Redis ready hits do not query PostgreSQL; only a missing snapshot enters the
// bounded cross-instance rebuild path. The rate-limit catalog is deliberately
// not loaded here: it belongs to Prepare and is irrelevant to readiness.
func (s *Service) VerifyCodeReady(ctx context.Context, scene string) (VerifyCodeReadiness, error) {
	if scene != SceneLogin {
		return VerifyCodeReadiness{}, nil
	}
	if s.readinessStore == nil {
		return VerifyCodeReadiness{}, dependency(fmt.Errorf("mail verification readiness store unavailable"))
	}
	readiness, err := s.readinessStore.Current(ctx, scene)
	if err != nil {
		return VerifyCodeReadiness{}, dependency(err)
	}
	return readiness, nil
}

// PrepareEmailVerifyCode runs readiness, recipient-rule and business
// rate-limit checks for one login verification email without sending anything.
// A successful call consumes the current business rate-limit allowance exactly
// once; the returned preparation must be reused by SendPreparedEmailVerifyCode.
func (s *Service) PrepareEmailVerifyCode(ctx context.Context, in EmailVerifyCodePrepareInput) (EmailVerifyCodePreparation, error) {
	email, err := recipientrule.NormalizeRecipient(in.ToEmail)
	if err != nil {
		return EmailVerifyCodePreparation{}, invalid(err)
	}
	if !mailtemplate.IsVerificationScene(in.Scene) {
		return EmailVerifyCodePreparation{}, invalid(fmt.Errorf("verification code scene is invalid"))
	}
	if s.readinessStore == nil {
		return EmailVerifyCodePreparation{}, dependency(fmt.Errorf("mail verification readiness store unavailable"))
	}
	readiness, err := s.readinessStore.Current(ctx, in.Scene)
	if err != nil {
		return EmailVerifyCodePreparation{}, dependency(err)
	}
	if !readiness.Ready {
		return EmailVerifyCodePreparation{}, dependency(fmt.Errorf("mail verification is unavailable"))
	}
	if readiness.TTLMinutes < 1 || readiness.TTLMinutes > verifyCodeReadinessTTLMaximum {
		return EmailVerifyCodePreparation{}, dependency(fmt.Errorf("mail verification readiness TTL is invalid"))
	}
	var decision RuleDecision
	var evaluateErr error
	if s.runtime != nil {
		runtime, runtimeErr := s.runtime.Load(ctx, in.Scene)
		if runtimeErr != nil {
			return EmailVerifyCodePreparation{}, dependency(runtimeErr)
		}
		decision, evaluateErr = recipientrule.EvaluateRows(runtime.Rules, email)
	} else {
		if s.rules == nil {
			return EmailVerifyCodePreparation{}, dependency(fmt.Errorf("mail recipient rule evaluator unavailable"))
		}
		decision, evaluateErr = s.rules.Evaluate(ctx, email, SendModeBusiness)
	}
	if evaluateErr != nil {
		return EmailVerifyCodePreparation{}, dependency(evaluateErr)
	}
	if !decision.Allowed {
		return EmailVerifyCodePreparation{}, denied(ErrRecipientDenied)
	}
	catalog, err := s.loadRateLimitCatalog(ctx, in.PlatformID)
	if err != nil {
		return EmailVerifyCodePreparation{}, dependency(err)
	}
	if _, ok := ratelimitpolicy.Find(catalog, "business_email_minute"); !ok {
		return EmailVerifyCodePreparation{}, dependency(fmt.Errorf("mail resend rate-limit policy is invalid"))
	}
	reservation, err := s.reserveEmail(ctx, catalog, in.PlatformID, email)
	if err != nil {
		return EmailVerifyCodePreparation{}, err
	}
	return EmailVerifyCodePreparation{TTLMinutes: readiness.TTLMinutes, ResendAfterSeconds: reservation.RetryAfterSeconds}, nil
}

// SendPreparedEmailVerifyCode sends a six-digit login verification email using
// a previously produced preparation. It must not rate-limit a second time and
// persists the exact caller-supplied expiry.
func (s *Service) SendPreparedEmailVerifyCode(ctx context.Context, in EmailVerifyCodeInput) (EmailVerifyCodeResult, error) {
	email, err := recipientrule.NormalizeRecipient(in.ToEmail)
	if err != nil {
		return EmailVerifyCodeResult{}, invalid(err)
	}
	if !mailtemplate.IsVerificationScene(in.Scene) {
		return EmailVerifyCodeResult{}, invalid(fmt.Errorf("verification code scene is invalid"))
	}
	if !isSixDigitCode(in.Code) {
		return EmailVerifyCodeResult{}, invalid(fmt.Errorf("verification code must be six digits"))
	}
	if in.Preparation.TTLMinutes < 1 || in.Preparation.TTLMinutes > verifyCodeReadinessTTLMaximum {
		return EmailVerifyCodeResult{}, invalid(fmt.Errorf("verification code preparation TTL is invalid"))
	}
	if in.Preparation.ResendAfterSeconds < 0 || in.Preparation.ResendAfterSeconds > 86400 {
		return EmailVerifyCodeResult{}, invalid(fmt.Errorf("verification code resend wait is invalid"))
	}
	now := time.Now().UTC()
	if in.ExpiresAt.IsZero() || !in.ExpiresAt.After(now) {
		return EmailVerifyCodeResult{}, invalid(fmt.Errorf("verification code expiry is invalid"))
	}
	if in.ExpiresAt.After(now.Add(time.Duration(in.Preparation.TTLMinutes) * time.Minute)) {
		return EmailVerifyCodeResult{}, invalid(fmt.Errorf("verification code expiry exceeds preparation TTL"))
	}
	result, err := s.sendInternal(ctx, BusinessSendInput{
		PlatformID:            in.PlatformID,
		UserID:                in.UserID,
		ClientIP:              in.ClientIP,
		ChallengeID:           in.ChallengeID,
		Scene:                 in.Scene,
		ToEmail:               email,
		Variables:             map[string]string{"code": in.Code, "ttl_minutes": strconv.Itoa(in.Preparation.TTLMinutes)},
		RejectActiveChallenge: true,
	}, SendModeBusiness, sendInternalOptions{skipPreflight: true, expiresAt: in.ExpiresAt, ttlMinutes: in.Preparation.TTLMinutes})
	if err != nil {
		return EmailVerifyCodeResult{}, err
	}
	return EmailVerifyCodeResult{
		LogID:       result.LogID,
		ChallengeID: in.ChallengeID,
		ExpiresAt:   in.ExpiresAt,
	}, nil
}

func isSixDigitCode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

type sendInternalOptions struct {
	skipPreflight bool
	expiresAt     time.Time
	ttlMinutes    int
}

func (s *Service) send(ctx context.Context, in BusinessSendInput, mode SendMode) (SendResult, error) {
	return s.sendInternal(ctx, in, mode, sendInternalOptions{})
}

func (s *Service) sendInternal(ctx context.Context, in BusinessSendInput, mode SendMode, opts sendInternalOptions) (SendResult, error) {
	if in.PlatformID < 1 {
		return SendResult{}, invalid(fmt.Errorf("platform invalid"))
	}
	email, e := recipientrule.NormalizeRecipient(in.ToEmail)
	if e != nil {
		return SendResult{}, invalid(e)
	}
	_, ok := fixedTemplate(in.Scene)
	if !ok {
		return SendResult{}, invalid(fmt.Errorf("scene invalid"))
	}
	if e := mailtemplate.ValidateVariables(in.Variables, true); e != nil {
		return SendResult{}, invalid(e)
	}
	var runtime runtimeSnapshot
	if s.runtime != nil {
		var runtimeErr error
		runtime, runtimeErr = s.runtime.Load(ctx, in.Scene)
		if runtimeErr != nil {
			return SendResult{}, dependency(runtimeErr)
		}
	}
	if !opts.skipPreflight {
		if s.runtime != nil {
			d, e := recipientrule.EvaluateRows(runtime.Rules, email)
			if e != nil {
				return SendResult{}, dependency(e)
			}
			if !d.Allowed {
				return SendResult{}, denied(ErrRecipientDenied)
			}
		} else if s.rules != nil {
			d, e := s.rules.Evaluate(ctx, email, mode)
			if e != nil {
				return SendResult{}, dependency(e)
			}
			if !d.Allowed {
				return SendResult{}, denied(ErrRecipientDenied)
			}
		}
		{
			catalog, err := s.loadRateLimitCatalog(ctx, in.PlatformID)
			if err != nil {
				return SendResult{}, dependency(err)
			}
			if err := s.allowEmail(ctx, catalog, in.PlatformID, email); err != nil {
				return SendResult{}, err
			}
		}
	}
	if in.ChallengeID != "" {
		if old, e := s.stores.Log.FindActiveChallenge(ctx, in.PlatformID, in.ChallengeID); e == nil {
			if in.RejectActiveChallenge {
				return SendResult{}, conflict(fmt.Errorf("mail challenge is already active"))
			}
			return sendResultFromLog(old), nil
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return SendResult{}, wrapRepo(e)
		}
	}
	c := runtime.Config
	if s.runtime == nil {
		var configErr error
		var config Config
		config, configErr = s.stores.Config.Find(ctx)
		if configErr != nil {
			return SendResult{}, wrapRepo(configErr)
		}
		c = runtimeConfigFromModel(config)
	}
	if c.IsEnabled != yesno.Yes {
		return SendResult{}, dependency(fmt.Errorf("mail config disabled"))
	}
	templates := runtime.Templates
	if s.runtime == nil {
		var templateErr error
		templates, templateErr = s.stores.Template.List(ctx)
		if templateErr != nil {
			return SendResult{}, wrapRepo(templateErr)
		}
	}
	var tpl Template
	for _, t := range templates {
		if t.Scene == in.Scene {
			tpl = t
		}
	}
	if tpl.ID == 0 || tpl.IsEnabled != yesno.Yes {
		return SendResult{}, dependency(fmt.Errorf("mail template disabled"))
	}
	variables := make(map[string]string, len(in.Variables))
	for key, value := range in.Variables {
		variables[key] = value
	}
	effectiveTTLMinutes := int(c.TTLMinutes)
	if opts.ttlMinutes != 0 {
		effectiveTTLMinutes = opts.ttlMinutes
	}
	if effectiveTTLMinutes < 1 || effectiveTTLMinutes > verifyCodeReadinessTTLMaximum {
		return SendResult{}, dependency(fmt.Errorf("mail config TTL is invalid"))
	}
	variables["ttl_minutes"] = strconv.Itoa(effectiveTTLMinutes)
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(effectiveTTLMinutes) * time.Minute)
	if !opts.expiresAt.IsZero() {
		expiresAt = opts.expiresAt
	}
	var challengePtr *string
	if in.ChallengeID != "" {
		challengePtr = &in.ChallengeID
	}
	row, e := s.stores.Log.CreatePending(ctx, &Log{PlatformID: in.PlatformID, ChallengeID: challengePtr, UserID: in.UserID, Scene: in.Scene, TemplateID: tpl.TencentTemplateID, ToEmail: email, Subject: tpl.Subject, Status: StatusPending, CreatedAt: now, UpdatedAt: now})
	if e != nil {
		if in.ChallengeID != "" && isUniqueViolation(e) {
			if old, findErr := s.stores.Log.FindActiveChallenge(ctx, in.PlatformID, in.ChallengeID); findErr == nil {
				if in.RejectActiveChallenge {
					return SendResult{}, conflict(fmt.Errorf("mail challenge is already active"))
				}
				return sendResultFromLog(old), nil
			} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return SendResult{}, wrapRepo(findErr)
			}
		}
		return SendResult{}, wrapRepo(e)
	}
	sendStarted := time.Now()
	if s.keys != nil {
		ct, ver, ce := secretkey.EncryptMailValue(s.keys.MailEncryptionKey(), variables["code"])
		if ce != nil {
			return s.failPending(ctx, in.PlatformID, row.ID, ce, sendStarted)
		}
		if ve := s.stores.LogVerification.Create(ctx, &Verification{PlatformID: in.PlatformID, MailLogID: row.ID, KeyVersion: ver, CodeCiphertext: ct, ExpiresAt: expiresAt, CreatedAt: now}); ve != nil {
			return s.failPending(ctx, in.PlatformID, row.ID, ve, sendStarted)
		}
	}
	if s.sender == nil {
		sendErr := fmt.Errorf("sender unavailable")
		return s.failPending(ctx, in.PlatformID, row.ID, sendErr, sendStarted)
	}
	sendContext, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	secretID, decryptErr := decryptMailSecret(s.keys, c.SecretIDCiphertext)
	if decryptErr != nil {
		return s.failPending(ctx, in.PlatformID, row.ID, decryptErr, sendStarted)
	}
	secretKey, decryptErr := decryptMailSecret(s.keys, c.SecretKeyCiphertext)
	if decryptErr != nil {
		return s.failPending(ctx, in.PlatformID, row.ID, decryptErr, sendStarted)
	}
	result, se := s.sender.Send(sendContext, SendInput{Region: c.Region, Endpoint: pointerValue(c.Endpoint), SecretID: secretID, SecretKey: secretKey, FromEmail: c.FromEmail, FromName: c.FromName, ReplyTo: pointerValue(c.ReplyTo), ToEmail: email, Subject: tpl.Subject, TemplateID: tpl.TencentTemplateID, TemplateData: variables})
	if se != nil {
		return s.failPending(ctx, in.PlatformID, row.ID, se, sendStarted)
	}
	statusCtx, cancelStatus := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancelStatus()
	if me := s.stores.Log.MarkSent(statusCtx, in.PlatformID, row.ID, maillog.ProviderResult(result), time.Since(sendStarted).Milliseconds()); me != nil {
		return SendResult{LogID: row.ID, Status: StatusPending, RequestID: result.RequestID, MessageID: result.MessageID}, dependency(fmt.Errorf("persist sent mail status: %w", me))
	}
	return SendResult{LogID: row.ID, Status: StatusSent, RequestID: result.RequestID, MessageID: result.MessageID}, nil
}

func sendResultFromLog(log Log) SendResult {
	return SendResult{LogID: log.ID, Status: log.Status, RequestID: log.RequestID, MessageID: log.MessageID}
}

func (s *Service) failPending(ctx context.Context, platformID, logID int64, cause error, started time.Time) (SendResult, error) {
	providerErrorValue := providerError(cause)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	if me := s.stores.Log.MarkFailed(cleanupCtx, platformID, logID, providerErrorValue.Code, providerErrorValue.Summary, time.Since(started).Milliseconds()); me != nil {
		return SendResult{LogID: logID, Status: StatusPending}, dependency(fmt.Errorf("persist failed mail status: %w", me))
	}
	if _, ok := cause.(*ProviderError); ok {
		return SendResult{LogID: logID, Status: StatusFailed}, providerFailure(cause)
	}
	return SendResult{LogID: logID, Status: StatusFailed}, dependency(cause)
}
func (s *Service) TestForPlatform(ctx context.Context, platformID int64, in AdminTestInput) (AdminTestResult, error) {
	r, err := s.send(ctx, BusinessSendInput{
		PlatformID: platformID, UserID: &in.AdminUserID, ClientIP: in.ClientIP,
		Scene: in.Scene, ToEmail: in.ToEmail, Variables: in.Variables,
	}, SendModeAdminTest)
	result := AdminTestResult{LogID: r.LogID, Status: r.Status, RequestID: r.RequestID, MessageID: r.MessageID}
	if s.stores == nil || s.stores.Config == nil {
		if err == nil {
			err = dependency(fmt.Errorf("mail repository unavailable"))
		}
		return result, err
	}
	if recordErr := s.stores.Config.RecordTestResult(ctx, time.Now().UTC(), errorSummary(err)); recordErr != nil && err == nil {
		err = dependency(fmt.Errorf("persist mail test result: %w", recordErr))
	}
	return result, err
}

func errorSummary(err error) string {
	if err == nil {
		return ""
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return truncateErrorSummary(providerErr.Summary)
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) && appErr.Cause != nil {
		return truncateErrorSummary(appErr.Cause.Error())
	}
	return truncateErrorSummary(err.Error())
}

func truncateErrorSummary(value string) string {
	if len(value) > 512 {
		return value[:512]
	}
	return value
}
func fixedTemplate(scene string) (mailtemplate.Fixed, bool) { return mailtemplate.FindFixed(scene) }
func decryptMailSecret(k *secretkey.KeyRing, ct string) (string, error) {
	if k == nil {
		return "", fmt.Errorf("mail encryption key unavailable")
	}
	v, err := secretkey.DecryptMailValue(k.MailEncryptionKey(), ct)
	if err != nil || v == "" {
		if err == nil {
			err = fmt.Errorf("mail secret is empty")
		}
		return "", err
	}
	return v, nil
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func (s *Service) allowEmail(ctx context.Context, catalog RateLimitCatalog, platformID int64, email string) error {
	_, err := s.reserveEmail(ctx, catalog, platformID, email)
	return err
}

func (s *Service) reserveEmail(ctx context.Context, catalog RateLimitCatalog, platformID int64, email string) (LimitResult, error) {
	if s.limiter == nil {
		return LimitResult{}, dependency(fmt.Errorf("mail rate limiter unavailable"))
	}
	if s.keys == nil || len(s.keys.MailRecipientHMACKey()) == 0 {
		return LimitResult{}, dependency(fmt.Errorf("mail recipient HMAC key unavailable"))
	}
	recipientKey := mailRecipientKey(s.keys, email)
	result, err := s.limiter.Reserve(ctx, businessLimitRequests(catalog, platformID, recipientKey)...)
	if err != nil {
		return LimitResult{}, dependency(err)
	}
	if !result.Allowed {
		return LimitResult{}, rateLimited(ErrRateLimited)
	}
	return result, nil
}

func mailRecipientKey(keys *secretkey.KeyRing, email string) string {
	digest := hmac.New(sha256.New, keys.MailRecipientHMACKey())
	_, _ = digest.Write([]byte(email))
	return hex.EncodeToString(digest.Sum(nil))
}

func (s *Service) loadRateLimitCatalog(ctx context.Context, platformID int64) (RateLimitCatalog, error) {
	if s.policyStore == nil {
		return RateLimitCatalog{}, fmt.Errorf("mail rate limit policy store unavailable")
	}
	return s.policyStore.Load(ctx, platformID)
}

func businessLimitRequests(catalog RateLimitCatalog, platformID int64, email string) []LimitRequest {
	limits := policyLimitMap(catalog)
	requests := make([]LimitRequest, 0, 2)
	for _, spec := range []struct{ key, prefix string }{{"business_email_minute", "email"}, {"business_email_10m", "email10"}} {
		policy := limits[spec.key]
		requests = append(requests, LimitRequest{Key: fmt.Sprintf("mail:send:%s:v2:%d:%s", spec.prefix, platformID, email), Limit: policy.Limit, Window: policyWindow(policy)})
	}
	return requests
}

func policyLimitMap(catalog RateLimitCatalog) map[string]RateLimitPolicy {
	limits := make(map[string]RateLimitPolicy, len(catalog.Policies))
	for _, policy := range catalog.Policies {
		limits[policy.Key] = policy
	}
	return limits
}

func policyWindow(policy RateLimitPolicy) time.Duration {
	return time.Duration(policy.WindowSeconds) * time.Second
}
