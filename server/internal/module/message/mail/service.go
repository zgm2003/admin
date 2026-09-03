package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Service struct {
	repository *Repository
	keys       *secretkey.KeyRing
	sender     Sender
	rules      RuleEvaluator
	limiter    Limiter
}

func NewService(r *Repository, keys *secretkey.KeyRing, sender Sender, rules RuleEvaluator, limiter Limiter) *Service {
	return &Service{repository: r, keys: keys, sender: sender, rules: rules, limiter: limiter}
}

type SafeConfig struct {
	Configured    bool        `json:"configured"`
	Region        string      `json:"region"`
	Endpoint      string      `json:"endpoint"`
	FromEmail     string      `json:"fromEmail"`
	FromName      string      `json:"fromName"`
	ReplyTo       string      `json:"replyTo"`
	TTLMinutes    int         `json:"ttlMinutes"`
	IsEnabled     yesno.Value `json:"isEnabled"`
	LastTestAt    *time.Time  `json:"lastTestAt"`
	LastTestError string      `json:"lastTestError"`
}

func safeConfig(c Config) SafeConfig {
	return SafeConfig{Configured: c.SecretIDCiphertext != "" && c.SecretKeyCiphertext != "", Region: c.Region, Endpoint: stringValue(c.Endpoint), FromEmail: c.FromEmail, FromName: c.FromName, ReplyTo: stringValue(c.ReplyTo), TTLMinutes: int(c.TTLMinutes), IsEnabled: c.IsEnabled, LastTestAt: c.LastTestAt, LastTestError: c.LastTestError}
}
func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (s *Service) GetConfig(ctx context.Context, platformID int64) (SafeConfig, error) {
	c, e := s.repository.FindConfig(ctx, platformID)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return SafeConfig{Configured: false, TTLMinutes: 10, IsEnabled: yesno.No}, nil
	}
	if e != nil {
		return SafeConfig{}, wrapRepo(e)
	}
	return safeConfig(c), nil
}
func (s *Service) SaveConfig(ctx context.Context, platformID int64, in ConfigInput) (SafeConfig, error) {
	if platformID < 1 || in.TTLMinutes < 1 || in.TTLMinutes > 60 || !yesno.IsValid(in.IsEnabled) {
		return SafeConfig{}, invalid(fmt.Errorf("invalid mail config"))
	}
	fromEmail, e := normalizeMailConfigEmail(in.FromEmail)
	if e != nil {
		return SafeConfig{}, invalid(e)
	}
	in.FromEmail = fromEmail
	if strings.TrimSpace(in.Region) == "" || strings.TrimSpace(in.FromName) == "" {
		return SafeConfig{}, invalid(fmt.Errorf("region and fromName are required"))
	}
	if s.keys == nil {
		return SafeConfig{}, dependency(fmt.Errorf("mail encryption key unavailable"))
	}
	if strings.TrimSpace(in.SecretID) == "" || strings.TrimSpace(in.SecretKey) == "" {
		current, err := s.repository.FindConfig(ctx, platformID)
		if err != nil {
			return SafeConfig{}, invalid(fmt.Errorf("credentials are required for the first configuration"))
		}
		if strings.TrimSpace(in.SecretID) == "" {
			in.SecretID = mustDecrypt(s.keys, current.SecretIDCiphertext)
		}
		if strings.TrimSpace(in.SecretKey) == "" {
			in.SecretKey = mustDecrypt(s.keys, current.SecretKeyCiphertext)
		}
	}
	if strings.TrimSpace(in.SecretID) == "" || strings.TrimSpace(in.SecretKey) == "" {
		return SafeConfig{}, invalid(fmt.Errorf("credentials are required"))
	}
	if strings.TrimSpace(in.ReplyTo) != "" {
		replyTo, e := normalizeMailConfigEmail(in.ReplyTo)
		if e != nil {
			return SafeConfig{}, invalid(e)
		}
		in.ReplyTo = replyTo
	}
	sid, _, e := EncryptSecret(s.keys.MailEncryptionKey(), in.SecretID)
	if e != nil {
		return SafeConfig{}, dependency(e)
	}
	skey, _, e := EncryptSecret(s.keys.MailEncryptionKey(), in.SecretKey)
	if e != nil {
		return SafeConfig{}, dependency(e)
	}
	v := map[string]any{"secret_id_ciphertext": sid, "secret_key_ciphertext": skey, "secret_id_hint": hint(in.SecretID), "secret_key_hint": hint(in.SecretKey), "region": strings.TrimSpace(in.Region), "from_email": strings.TrimSpace(in.FromEmail), "from_name": strings.TrimSpace(in.FromName), "endpoint": nullableText(in.Endpoint), "reply_to": nullableText(in.ReplyTo), "ttl_minutes": in.TTLMinutes, "is_enabled": in.IsEnabled, "updated_at": time.Now().UTC()}
	c, e := s.repository.SaveConfig(ctx, platformID, v)
	if e != nil {
		return SafeConfig{}, wrapRepo(e)
	}
	return safeConfig(c), nil
}
func nullableText(value string) any {
	if value = strings.TrimSpace(value); value == "" {
		return nil
	}
	return value
}
func hint(v string) string {
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + "***" + v[len(v)-2:]
}
func (s *Service) DeleteConfig(ctx context.Context, p int64) error {
	e := s.repository.DeleteConfig(ctx, p)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return notFound(e)
	}
	if e != nil {
		return wrapRepo(e)
	}
	return nil
}

func (s *Service) ListTemplates(ctx context.Context, p int64) ([]Template, error) {
	v, e := s.repository.ListTemplates(ctx, p)
	if e != nil {
		return nil, wrapRepo(e)
	}
	return v, nil
}
func (s *Service) UpdateTemplate(ctx context.Context, p, id int64, in TemplateUpdateInput) error {
	fixed, ok := fixedTemplate(in.Scene)
	if !ok {
		return invalid(fmt.Errorf("scene is invalid"))
	}
	if in.TencentTemplateID != fixed.TencentTemplateID {
		return invalid(fmt.Errorf("template id does not match scene"))
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Subject) == "" {
		return invalid(fmt.Errorf("template name and subject are required"))
	}
	if e := validateMailVariables(in.Variables, true); e != nil {
		return invalid(e)
	}
	if e := validateMailVariables(in.ExampleVariables, true); e != nil {
		return invalid(e)
	}
	current, e := s.repository.FindTemplate(ctx, p, id)
	if e != nil {
		return wrapRepo(e)
	}
	if current.Scene != in.Scene {
		return invalid(fmt.Errorf("template scene cannot be changed"))
	}
	values, e := templateUpdateValues(in)
	if e != nil {
		return invalid(e)
	}
	values["updated_at"] = time.Now().UTC()
	e = s.repository.UpdateTemplate(ctx, p, id, values)
	if e != nil {
		return wrapRepo(e)
	}
	return nil
}

func templateUpdateValues(in TemplateUpdateInput) (map[string]any, error) {
	variables, err := json.Marshal(in.Variables)
	if err != nil {
		return nil, fmt.Errorf("encode template variables: %w", err)
	}
	examples, err := json.Marshal(in.ExampleVariables)
	if err != nil {
		return nil, fmt.Errorf("encode example variables: %w", err)
	}
	return map[string]any{
		"name": in.Name, "subject": in.Subject, "tencent_template_id": in.TencentTemplateID,
		"variables": variables, "example_variables": examples,
	}, nil
}
func (s *Service) SetTemplateStatus(ctx context.Context, p, id int64, v yesno.Value) error {
	if !yesno.IsValid(v) {
		return invalid(fmt.Errorf("status invalid"))
	}
	if e := s.repository.UpdateTemplate(ctx, p, id, map[string]any{"is_enabled": v, "updated_at": time.Now().UTC()}); e != nil {
		return wrapRepo(e)
	}
	return nil
}

func (s *Service) Send(ctx context.Context, in BusinessSendInput) (SendResult, error) {
	return s.send(ctx, in, SendModeBusiness)
}

func (s *Service) send(ctx context.Context, in BusinessSendInput, mode SendMode) (SendResult, error) {
	if in.PlatformID < 1 {
		return SendResult{}, invalid(fmt.Errorf("platform invalid"))
	}
	email, e := NormalizeRecipient(in.ToEmail)
	if e != nil {
		return SendResult{}, invalid(e)
	}
	fixed, ok := fixedTemplate(in.Scene)
	if !ok {
		return SendResult{}, invalid(fmt.Errorf("scene invalid"))
	}
	if e := validateMailVariables(in.Variables, true); e != nil {
		return SendResult{}, invalid(e)
	}
	if s.rules != nil {
		d, e := s.rules.Evaluate(ctx, in.PlatformID, email, mode)
		if e != nil {
			return SendResult{}, dependency(e)
		}
		if !d.Allowed {
			return SendResult{}, denied(ErrRecipientDenied)
		}
	}
	if mode == SendModeBusiness {
		if !s.allow(ctx, fmt.Sprintf("mail:send:email:%d:%s:%s", in.PlatformID, in.Scene, email), 1, time.Minute) {
			return SendResult{}, rateLimited(ErrRateLimited)
		}
		if !s.allow(ctx, fmt.Sprintf("mail:send:email10:%d:%s:%s", in.PlatformID, in.Scene, email), 5, 10*time.Minute) || !s.allow(ctx, fmt.Sprintf("mail:send:ip:%d:%s", in.PlatformID, in.ClientIP), 10, time.Minute) || !s.allow(ctx, fmt.Sprintf("mail:send:scene:%d:%s", in.PlatformID, in.Scene), 30, time.Minute) {
			return SendResult{}, rateLimited(ErrRateLimited)
		}
	}
	if in.ChallengeID != "" {
		if old, e := s.repository.FindActiveChallenge(ctx, in.PlatformID, in.ChallengeID); e == nil {
			return sendResultFromLog(old), nil
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return SendResult{}, wrapRepo(e)
		}
	}
	c, e := s.repository.FindConfig(ctx, in.PlatformID)
	if e != nil {
		return SendResult{}, wrapRepo(e)
	}
	if c.IsEnabled != yesno.Yes {
		return SendResult{}, dependency(fmt.Errorf("mail config disabled"))
	}
	templates, e := s.repository.ListTemplates(ctx, in.PlatformID)
	if e != nil {
		return SendResult{}, wrapRepo(e)
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
	variables["ttl_minutes"] = strconv.Itoa(int(c.TTLMinutes))
	now := time.Now().UTC()
	var challengePtr *string
	if in.ChallengeID != "" {
		challengePtr = &in.ChallengeID
	}
	row, e := s.repository.CreatePendingLog(ctx, &Log{PlatformID: in.PlatformID, ChallengeID: challengePtr, UserID: in.UserID, Scene: in.Scene, TemplateID: fixed.TencentTemplateID, ToEmail: email, Subject: tpl.Subject, Status: StatusPending, CreatedAt: now, UpdatedAt: now})
	if e != nil {
		if in.ChallengeID != "" && isUniqueViolation(e) {
			if old, findErr := s.repository.FindActiveChallenge(ctx, in.PlatformID, in.ChallengeID); findErr == nil {
				return sendResultFromLog(old), nil
			} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return SendResult{}, wrapRepo(findErr)
			}
		}
		return SendResult{}, wrapRepo(e)
	}
	sendStarted := time.Now()
	if s.keys != nil {
		ct, ver, ce := EncryptSecret(s.keys.MailEncryptionKey(), variables["code"])
		if ce != nil {
			return s.failPending(ctx, in.PlatformID, row.ID, ce, sendStarted)
		}
		if ve := s.repository.AddVerification(ctx, &Verification{PlatformID: in.PlatformID, MailLogID: row.ID, KeyVersion: ver, CodeCiphertext: ct, ExpiresAt: now.Add(time.Duration(c.TTLMinutes) * time.Minute), CreatedAt: now}); ve != nil {
			return s.failPending(ctx, in.PlatformID, row.ID, ve, sendStarted)
		}
	}
	if s.sender == nil {
		sendErr := fmt.Errorf("sender unavailable")
		return s.failPending(ctx, in.PlatformID, row.ID, sendErr, sendStarted)
	}
	sendContext, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result, se := s.sender.Send(sendContext, SendInput{Region: c.Region, Endpoint: stringValue(c.Endpoint), SecretID: mustDecrypt(s.keys, c.SecretIDCiphertext), SecretKey: mustDecrypt(s.keys, c.SecretKeyCiphertext), FromEmail: c.FromEmail, FromName: c.FromName, ReplyTo: stringValue(c.ReplyTo), ToEmail: email, Subject: tpl.Subject, TemplateID: fixed.TencentTemplateID, TemplateData: variables})
	if se != nil {
		return s.failPending(ctx, in.PlatformID, row.ID, se, sendStarted)
	}
	if me := s.repository.MarkSent(ctx, in.PlatformID, row.ID, result, time.Since(sendStarted).Milliseconds()); me != nil {
		return SendResult{LogID: row.ID, Status: StatusPending, RequestID: result.RequestID, MessageID: result.MessageID}, dependency(fmt.Errorf("persist sent mail status: %w", me))
	}
	return SendResult{LogID: row.ID, Status: StatusSent, RequestID: result.RequestID, MessageID: result.MessageID}, nil
}

func sendResultFromLog(log Log) SendResult {
	return SendResult{LogID: log.ID, Status: log.Status, RequestID: log.RequestID, MessageID: log.MessageID}
}

func (s *Service) failPending(ctx context.Context, platformID, logID int64, cause error, started time.Time) (SendResult, error) {
	if me := s.repository.MarkFailed(ctx, platformID, logID, providerError(cause), time.Since(started).Milliseconds()); me != nil {
		return SendResult{LogID: logID, Status: StatusPending}, dependency(fmt.Errorf("persist failed mail status: %w", me))
	}
	if _, ok := cause.(*ProviderError); ok {
		return SendResult{LogID: logID, Status: StatusFailed}, providerFailure(cause)
	}
	return SendResult{LogID: logID, Status: StatusFailed}, dependency(cause)
}
func (s *Service) Test(ctx context.Context, in AdminTestInput) (AdminTestResult, error) {
	return s.TestForPlatform(ctx, 1, in)
}
func (s *Service) TestForPlatform(ctx context.Context, platformID int64, in AdminTestInput) (AdminTestResult, error) {
	email, err := NormalizeRecipient(in.ToEmail)
	if err != nil {
		return AdminTestResult{}, invalid(err)
	}
	if !s.allow(ctx, fmt.Sprintf("mail:test:user:%d", in.AdminUserID), 5, 10*time.Minute) || !s.allow(ctx, fmt.Sprintf("mail:test:ip:%s", in.ClientIP), 10, time.Minute) || !s.allow(ctx, fmt.Sprintf("mail:test:email:%s", email), 3, 10*time.Minute) {
		return AdminTestResult{}, rateLimited(ErrRateLimited)
	}
	r, e := s.send(ctx, BusinessSendInput{PlatformID: platformID, UserID: &in.AdminUserID, ClientIP: in.ClientIP, Scene: in.Scene, ToEmail: in.ToEmail, Variables: in.Variables}, SendModeAdminTest)
	if recordErr := s.repository.RecordTestResult(ctx, platformID, time.Now().UTC(), errorSummary(e)); recordErr != nil && e == nil {
		e = dependency(fmt.Errorf("persist mail test result: %w", recordErr))
	}
	return AdminTestResult{LogID: r.LogID, Status: r.Status, RequestID: r.RequestID, MessageID: r.MessageID}, e
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
func fixedTemplate(scene string) (FixedTemplate, bool) {
	for _, v := range FixedTemplates() {
		if v.Scene == scene {
			return v, true
		}
	}
	return FixedTemplate{}, false
}
func mustDecrypt(k *secretkey.KeyRing, ct string) string {
	if k == nil {
		return ""
	}
	v, _ := DecryptSecret(k.MailEncryptionKey(), ct)
	return v
}
func (s *Service) allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	if s.limiter == nil {
		return false
	}
	ok, e := s.limiter.Allow(ctx, LimitRequest{Key: key, Limit: limit, Window: window})
	return e == nil && ok
}
func (s *Service) ListLogs(ctx context.Context, p int64, page, size int) ([]Log, int64, error) {
	rows, total, err := s.repository.ListLogs(ctx, p, page, size)
	if err != nil {
		return nil, 0, wrapRepo(err)
	}
	return rows, total, nil
}

type LogDetail struct {
	Log                   Log        `json:"log"`
	VerificationCode      string     `json:"verificationCode"`
	VerificationExpiresAt *time.Time `json:"verificationExpiresAt"`
}

func (s *Service) GetLogDetail(ctx context.Context, p, id int64) (LogDetail, error) {
	log, verification, err := s.repository.GetLogDetail(ctx, p, id)
	if err != nil {
		return LogDetail{}, wrapRepo(err)
	}
	result := LogDetail{Log: log}
	if verification.ID == 0 {
		return result, nil
	}
	if s.keys == nil {
		return LogDetail{}, dependency(fmt.Errorf("mail encryption key unavailable"))
	}
	code, err := DecryptSecret(s.keys.MailEncryptionKey(), verification.CodeCiphertext)
	if err != nil {
		return LogDetail{}, dependency(err)
	}
	result.VerificationCode, result.VerificationExpiresAt = code, &verification.ExpiresAt
	return result, nil
}
func (s *Service) DeleteLog(ctx context.Context, p, id int64) error {
	return wrapRepo(s.repository.DeleteLog(ctx, p, id))
}
func (s *Service) DeleteLogs(ctx context.Context, p int64, ids []int64) error {
	return wrapRepo(s.repository.DeleteLogs(ctx, p, ids))
}
func (s *Service) ListRules(ctx context.Context, p int64) ([]RecipientRule, error) {
	rows, err := s.repository.ListRules(ctx, p)
	if err != nil {
		return nil, wrapRepo(err)
	}
	return rows, nil
}
func (s *Service) CreateRule(ctx context.Context, p int64, in RuleInput) (int64, error) {
	if in.Action != RuleActionAllow && in.Action != RuleActionDeny || !yesno.IsValid(in.IsEnabled) || strings.TrimSpace(in.Name) == "" {
		return 0, invalid(fmt.Errorf("invalid recipient rule"))
	}
	pattern, e := NormalizeRule(in.Scope, in.Pattern)
	if e != nil {
		return 0, invalid(e)
	}
	r := &RecipientRule{PlatformID: p, Scope: in.Scope, Pattern: pattern, Action: in.Action, Name: in.Name, Remark: in.Remark, IsEnabled: in.IsEnabled, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if e = s.repository.CreateRule(ctx, r); e != nil {
		return 0, wrapRepo(e)
	}
	return r.ID, nil
}
func (s *Service) UpdateRule(ctx context.Context, p, id int64, in RuleInput) error {
	if in.Action != RuleActionAllow && in.Action != RuleActionDeny || !yesno.IsValid(in.IsEnabled) || strings.TrimSpace(in.Name) == "" {
		return invalid(fmt.Errorf("invalid recipient rule"))
	}
	pattern, e := NormalizeRule(in.Scope, in.Pattern)
	if e != nil {
		return invalid(e)
	}
	return wrapRepo(s.repository.UpdateRule(ctx, p, id, map[string]any{"scope": in.Scope, "pattern": pattern, "action": in.Action, "name": in.Name, "remark": in.Remark, "is_enabled": in.IsEnabled, "updated_at": time.Now().UTC()}))
}

func validateMailVariables(values map[string]string, requireValues bool) error {
	if len(values) != 2 {
		return fmt.Errorf("mail variables must contain code and ttl_minutes")
	}
	for _, key := range []string{"code", "ttl_minutes"} {
		value, ok := values[key]
		if !ok || (requireValues && strings.TrimSpace(value) == "") {
			return fmt.Errorf("mail variable %s is required", key)
		}
	}
	ttl, err := strconv.Atoi(strings.TrimSpace(values["ttl_minutes"]))
	if err != nil || ttl < 1 || ttl > 60 {
		return fmt.Errorf("mail variable ttl_minutes is invalid")
	}
	return nil
}

func normalizeMailConfigEmail(value string) (string, error) {
	return NormalizeRecipient(value)
}
func (s *Service) SetRuleStatus(ctx context.Context, p, id int64, v yesno.Value) error {
	if !yesno.IsValid(v) {
		return invalid(fmt.Errorf("status invalid"))
	}
	return wrapRepo(s.repository.UpdateRule(ctx, p, id, map[string]any{"is_enabled": v, "updated_at": time.Now().UTC()}))
}
func (s *Service) DeleteRule(ctx context.Context, p, id int64) error {
	return wrapRepo(s.repository.DeleteRule(ctx, p, id))
}
