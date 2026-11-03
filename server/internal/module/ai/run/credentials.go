package run

import (
	"admin/server/internal/module/ai/provider"
	pr "admin/server/internal/module/ai/providerRequest"
	"context"
	"encoding/json"
	"strings"
	"time"
)

type CredentialLoader struct {
	repository *provider.GormRepository
	cipher     *provider.Cipher
}

func NewCredentialLoader(repository *provider.GormRepository, cipher *provider.Cipher) *CredentialLoader {
	return &CredentialLoader{repository, cipher}
}
func (l *CredentialLoader) Load(ctx context.Context, s RuntimeSnapshot) (pr.FrozenConfig, error) {
	var out pr.FrozenConfig
	if l == nil || l.repository == nil || l.cipher == nil {
		return out, ErrRuntimeUnavailable
	}
	var config RuntimeConfig
	if json.Unmarshal(s.Config, &config) != nil {
		return out, ErrInvalidSnapshot
	}
	v, e := l.repository.Version(ctx, s.ProviderVersionID)
	if e != nil {
		return out, e
	}
	if v.ProviderID != config.ProviderID || v.RequestTimeoutMS < 1000 || v.RequestTimeoutMS > 300000 {
		return out, ErrInvalidSnapshot
	}
	key, e := l.cipher.Open(v.APIKeyCiphertext, v.EncryptionKeyID)
	if e != nil {
		return out, e
	}
	headers := map[string]string{}
	if v.HeadersCiphertext != nil {
		raw, err := l.cipher.Open(*v.HeadersCiphertext, v.EncryptionKeyID)
		if err != nil {
			return out, err
		}
		if json.Unmarshal([]byte(raw), &headers) != nil {
			return out, ErrInvalidSnapshot
		}
	}
	var hosts []string
	if len(v.MediaHostAllowlist) > 0 && json.Unmarshal(v.MediaHostAllowlist, &hosts) != nil {
		return out, ErrInvalidSnapshot
	}
	for i := range hosts {
		hosts[i] = strings.ToLower(strings.TrimSpace(hosts[i]))
	}
	return pr.FrozenConfig{BaseURL: v.BaseURL, APIKey: key, Model: config.UpstreamModelID, Headers: headers, MediaHostAllowlist: hosts, Timeout: time.Duration(v.RequestTimeoutMS) * time.Millisecond}, nil
}
