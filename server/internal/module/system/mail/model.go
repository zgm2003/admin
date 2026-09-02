package mail

import (
	"encoding/json"
	"fmt"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const (
	SceneLogin          = "login"
	SceneForget         = "forget"
	SceneBindEmail      = "bind_email"
	SceneChangePassword = "change_password"
	StatusPending       = "pending"
	StatusSent          = "sent"
	StatusFailed        = "failed"
	RuleScopeEmail      = "email"
	RuleScopeDomain     = "domain"
	RuleActionAllow     = "allow"
	RuleActionDeny      = "deny"
)

type FixedTemplate struct {
	Scene             string   `json:"scene"`
	Name              string   `json:"name"`
	Subject           string   `json:"subject"`
	TencentTemplateID int      `json:"tencentTemplateId"`
	Variables         []string `json:"variables"`
}

func FixedTemplates() []FixedTemplate {
	vars := []string{"code", "ttl_minutes"}
	return []FixedTemplate{
		{Scene: SceneLogin, Name: "邮箱验证码登录", Subject: "登录验证码", TencentTemplateID: 47941, Variables: append([]string(nil), vars...)},
		{Scene: SceneForget, Name: "找回密码", Subject: "找回密码验证码", TencentTemplateID: 47942, Variables: append([]string(nil), vars...)},
		{Scene: SceneBindEmail, Name: "绑定/换绑邮箱", Subject: "绑定邮箱验证码", TencentTemplateID: 47943, Variables: append([]string(nil), vars...)},
		{Scene: SceneChangePassword, Name: "验证码改密", Subject: "修改密码验证码", TencentTemplateID: 47944, Variables: append([]string(nil), vars...)},
	}
}

func ValidateScene(value string) error {
	for _, item := range FixedTemplates() {
		if value == item.Scene {
			return nil
		}
	}
	return fmt.Errorf("invalid mail scene")
}

func ValidateStatus(value string) error {
	if value == StatusPending || value == StatusSent || value == StatusFailed {
		return nil
	}
	return fmt.Errorf("invalid mail status")
}

type Config struct {
	ID                  int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PlatformID          int64          `gorm:"column:platform_id;not null" json:"platformId"`
	SecretIDCiphertext  string         `gorm:"column:secret_id_ciphertext;not null" json:"-"`
	SecretKeyCiphertext string         `gorm:"column:secret_key_ciphertext;not null" json:"-"`
	SecretIDHint        string         `gorm:"column:secret_id_hint;type:varchar(32);not null" json:"secretIdHint"`
	SecretKeyHint       string         `gorm:"column:secret_key_hint;type:varchar(32);not null" json:"secretKeyHint"`
	Region              string         `gorm:"column:region;type:varchar(64);not null" json:"region"`
	Endpoint            *string        `gorm:"column:endpoint;type:varchar(255)" json:"endpoint"`
	FromEmail           string         `gorm:"column:from_email;type:varchar(254);not null" json:"fromEmail"`
	FromName            string         `gorm:"column:from_name;type:varchar(128);not null" json:"fromName"`
	ReplyTo             *string        `gorm:"column:reply_to;type:varchar(254)" json:"replyTo"`
	TTLMinutes          int16          `gorm:"column:ttl_minutes;not null" json:"ttlMinutes"`
	IsEnabled           yesno.Value    `gorm:"column:is_enabled;type:smallint;not null" json:"isEnabled"`
	LastTestAt          *time.Time     `gorm:"column:last_test_at;type:timestamptz" json:"lastTestAt"`
	LastTestError       string         `gorm:"column:last_test_error;type:varchar(512);not null" json:"lastTestError"`
	CreatedAt           time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Config) TableName() string { return "system_mail_config" }

type Template struct {
	ID                int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PlatformID        int64           `gorm:"column:platform_id;not null" json:"platformId"`
	Scene             string          `gorm:"column:scene;type:varchar(32);not null" json:"scene"`
	Name              string          `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Subject           string          `gorm:"column:subject;type:varchar(255);not null" json:"subject"`
	TencentTemplateID int             `gorm:"column:tencent_template_id;not null" json:"tencentTemplateId"`
	Variables         json.RawMessage `gorm:"column:variables;type:jsonb;not null" json:"variables"`
	ExampleVariables  json.RawMessage `gorm:"column:example_variables;type:jsonb;not null" json:"exampleVariables"`
	IsEnabled         yesno.Value     `gorm:"column:is_enabled;type:smallint;not null" json:"isEnabled"`
	CreatedAt         time.Time       `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time       `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt  `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Template) TableName() string { return "system_mail_template" }

type Log struct {
	ID           int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PlatformID   int64          `gorm:"column:platform_id;not null" json:"platformId"`
	ChallengeID  *string        `gorm:"column:challenge_id;type:varchar(128)" json:"-"`
	UserID       *int64         `gorm:"column:user_id" json:"userId"`
	Scene        string         `gorm:"column:scene;type:varchar(32);not null" json:"scene"`
	TemplateID   int            `gorm:"column:template_id;not null" json:"templateId"`
	ToEmail      string         `gorm:"column:to_email;type:varchar(254);not null" json:"toEmail"`
	Subject      string         `gorm:"column:subject;type:varchar(255);not null" json:"subject"`
	Status       string         `gorm:"column:status;type:varchar(16);not null" json:"status"`
	RequestID    string         `gorm:"column:request_id;type:varchar(128);not null" json:"requestId"`
	MessageID    string         `gorm:"column:message_id;type:varchar(128);not null" json:"messageId"`
	ErrorCode    string         `gorm:"column:error_code;type:varchar(128);not null" json:"errorCode"`
	ErrorSummary string         `gorm:"column:error_summary;type:varchar(512);not null" json:"errorSummary"`
	LatencyMs    int64          `gorm:"column:latency_ms;not null" json:"latencyMs"`
	SentAt       *time.Time     `gorm:"column:sent_at;type:timestamptz" json:"sentAt"`
	CreatedAt    time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Log) TableName() string { return "system_mail_log" }

type Verification struct {
	ID             int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PlatformID     int64          `gorm:"column:platform_id;not null" json:"platformId"`
	MailLogID      int64          `gorm:"column:mail_log_id;not null" json:"mailLogId"`
	KeyVersion     string         `gorm:"column:key_version;type:varchar(16);not null" json:"-"`
	CodeCiphertext string         `gorm:"column:code_ciphertext;not null" json:"-"`
	ExpiresAt      time.Time      `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (Verification) TableName() string { return "system_mail_log_verification" }

type RecipientRule struct {
	ID         int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PlatformID int64          `gorm:"column:platform_id;not null" json:"platformId"`
	Scope      string         `gorm:"column:scope;type:varchar(16);not null" json:"scope"`
	Pattern    string         `gorm:"column:pattern;type:varchar(254);not null" json:"pattern"`
	Action     string         `gorm:"column:action;type:varchar(16);not null" json:"action"`
	Name       string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Remark     string         `gorm:"column:remark;type:varchar(512);not null" json:"remark"`
	IsEnabled  yesno.Value    `gorm:"column:is_enabled;type:smallint;not null" json:"isEnabled"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;type:timestamptz" json:"-"`
}

func (RecipientRule) TableName() string { return "system_mail_recipient_rule" }
