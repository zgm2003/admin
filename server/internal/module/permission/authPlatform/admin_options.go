package authplatform

import (
	"admin/server/internal/module/auth/client"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
	"context"
)

type numberRange struct {
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}
type authPlatformFormDefaults struct {
	LoginTypes             []LoginType `json:"loginTypes"`
	AccessTTLSeconds       int         `json:"accessTTLSeconds"`
	RefreshTTLSeconds      int         `json:"refreshTTLSeconds"`
	SessionCacheTTLSeconds int         `json:"sessionCacheTTLSeconds"`
	AccessCacheTTLSeconds  int         `json:"accessCacheTTLSeconds"`
	BindDevice             yesno.Value `json:"bindDevice"`
	BindIP                 yesno.Value `json:"bindIP"`
	MaxSessions            int         `json:"maxSessions"`
	AllowRegister          yesno.Value `json:"allowRegister"`
	IsEnabled              yesno.Value `json:"isEnabled"`
}
type authPlatformAdminOptions struct {
	LoginTypes   []option.Option[LoginType] `json:"loginTypes"`
	Limits       map[string]numberRange     `json:"limits"`
	Defaults     authPlatformFormDefaults   `json:"defaults"`
	CodePattern  string                     `json:"codePattern"`
	NameMaxBytes int                        `json:"nameMaxBytes"`
}

func adminOptions(ctx context.Context) (authPlatformAdminOptions, error) {
	loginTypes, err := loginTypeOptions(ctx)
	if err != nil {
		return authPlatformAdminOptions{}, err
	}
	return authPlatformAdminOptions{
		LoginTypes: loginTypes,
		Limits: map[string]numberRange{
			"accessTTLSeconds":       {MinimumAccessTTLSeconds, MaximumAccessTTLSeconds},
			"refreshTTLSeconds":      {MinimumRefreshTTLSeconds, MaximumRefreshTTLSeconds},
			"sessionCacheTTLSeconds": {MinimumSessionCacheTTLSeconds, MaximumSessionCacheTTLSeconds},
			"accessCacheTTLSeconds":  {MinimumAccessCacheTTLSeconds, MaximumAccessCacheTTLSeconds},
			"maxSessions":            {0, MaximumSessions},
		},
		Defaults:    authPlatformFormDefaults{LoginTypes: []LoginType{LoginTypeEmail, LoginTypePassword}, AccessTTLSeconds: 900, RefreshTTLSeconds: 86400, SessionCacheTTLSeconds: 7200, AccessCacheTTLSeconds: 600, BindDevice: yesno.Yes, BindIP: yesno.No, MaxSessions: 1, AllowRegister: yesno.No, IsEnabled: yesno.Yes},
		CodePattern: authclient.PlatformCodePattern(), NameMaxBytes: MaximumNameBytes,
	}, nil
}

func loginTypeOptions(ctx context.Context) ([]option.Option[LoginType], error) {
	return option.ManyLocalized(ctx, []option.LocalizedEntry[LoginType]{
		{Value: LoginTypeEmail, Key: i18n.KeyAuthPlatformLoginEmail},
		{Value: LoginTypePhone, Key: i18n.KeyAuthPlatformLoginPhone},
		{Value: LoginTypePassword, Key: i18n.KeyAuthPlatformLoginPassword},
	})
}
