package mail

import (
	"context"

	mailconfig "admin/server/internal/module/message/mail/config"
	maillog "admin/server/internal/module/message/mail/log"
	logverification "admin/server/internal/module/message/mail/logVerification"
	ratelimitpolicy "admin/server/internal/module/message/mail/rateLimitPolicy"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	mailtemplate "admin/server/internal/module/message/mail/template"
	"gorm.io/gorm"
)

type Stores struct {
	Config          *mailconfig.Repository
	Template        *mailtemplate.Repository
	Log             *maillog.Repository
	LogVerification *logverification.Repository
	RateLimitPolicy *ratelimitpolicy.Repository
	RecipientRule   *recipientrule.Repository
}

func NewStores(database *gorm.DB) *Stores {
	return &Stores{
		Config:          mailconfig.NewRepository(database),
		Template:        mailtemplate.NewRepository(database),
		Log:             maillog.NewRepository(database),
		LogVerification: logverification.NewRepository(database),
		RateLimitPolicy: ratelimitpolicy.NewRepository(database),
		RecipientRule:   recipientrule.NewRepository(database),
	}
}

func (s *Stores) FindConfig(ctx context.Context) (Config, error) {
	return s.Config.Find(ctx)
}

func (s *Stores) FindTemplateByScene(ctx context.Context, scene string) (Template, error) {
	return s.Template.FindByScene(ctx, scene)
}

func (s *Stores) ListTemplates(ctx context.Context) ([]Template, error) {
	return s.Template.List(ctx)
}

func (s *Stores) ListRecipientRules(ctx context.Context) ([]RecipientRule, error) {
	return s.RecipientRule.List(ctx)
}
