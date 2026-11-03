package provider

import (
	ai "admin/server/internal/module/ai"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type CredentialCipher interface {
	Seal(string) (string, string, error)
}
type Service struct {
	repository Repository
	cipher     CredentialCipher
	runtime    RuntimeCoordinator
}

func NewService(r Repository, c CredentialCipher, runtime ...RuntimeCoordinator) *Service {
	s := &Service{repository: r, cipher: c}
	if len(runtime) > 0 {
		s.runtime = runtime[0]
	}
	return s
}
func (s *Service) Create(ctx context.Context, req CreateRequest, actorID int64) (*Provider, error) {
	if actorID <= 0 || !regexp.MustCompile(`^[a-z][a-zA-Z0-9_-]{0,63}$`).MatchString(req.Code) || req.APIKey == "" || len(req.APIKey) > 16384 || (req.Status != 0 && req.Status != 1) {
		return nil, ErrInvalidConfiguration
	}
	if err := validateSettings(req.Name, req.Description, req.BaseURL, req.Headers, req.MediaHostAllowlist, req.RequestTimeoutMS); err != nil {
		return nil, err
	}
	if s.cipher == nil {
		return nil, ErrInvalidConfiguration
	}
	enc, key, err := s.cipher.Seal(req.APIKey)
	if err != nil {
		return nil, err
	}
	headers, err := s.encryptHeaders(req.Headers)
	if err != nil {
		return nil, err
	}
	p := &Provider{Code: req.Code, Name: req.Name, Description: req.Description, Status: req.Status, CurrentVersion: 1, APIKeyConfigured: true, CreatedBy: actorID, UpdatedBy: actorID}
	v := &ProviderVersion{Version: 1, BaseURL: req.BaseURL, APIKeyCiphertext: enc, EncryptionKeyID: key, HeadersCiphertext: headers, MediaHostAllowlist: req.MediaHostAllowlist, RequestTimeoutMS: req.RequestTimeoutMS, CreatedBy: actorID}
	if err := s.mutate(ctx, func(repo Repository) error { return repo.Create(ctx, p, v) }); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *Service) Update(ctx context.Context, id int64, req UpdateRequest, actorID int64) (*Provider, error) {
	if actorID <= 0 || id <= 0 {
		return nil, ErrInvalidConfiguration
	}
	if err := validateSettings(req.Name, req.Description, req.BaseURL, req.Headers, req.MediaHostAllowlist, req.RequestTimeoutMS); err != nil {
		return nil, err
	}
	var out *Provider
	err := s.mutate(ctx, func(base Repository) error {
		return base.WithProvider(ctx, id, func(repo Repository) error {
			p, err := repo.Find(ctx, id)
			if err != nil {
				return err
			}
			old, err := repo.CurrentVersion(ctx, id)
			if err != nil {
				return err
			}
			next := *old
			next.ID = 0
			next.CreatedAt = time.Time{}
			next.UpdatedAt = time.Time{}
			next.Version++
			next.CreatedBy = actorID
			next.BaseURL = req.BaseURL
			next.RequestTimeoutMS = req.RequestTimeoutMS
			next.MediaHostAllowlist = req.MediaHostAllowlist
			if req.APIKey != "" {
				if s.cipher == nil {
					return ErrInvalidConfiguration
				}
				enc, key, err := s.cipher.Seal(req.APIKey)
				if err != nil {
					return err
				}
				next.APIKeyCiphertext, next.EncryptionKeyID = enc, key
			}
			if len(req.Headers) > 0 {
				headers, err := s.encryptHeaders(req.Headers)
				if err != nil {
					return err
				}
				next.HeadersCiphertext = headers
			}
			p.Name = req.Name
			p.Description = req.Description
			p.UpdatedBy = actorID
			p.CurrentVersion = next.Version
			if err := repo.SaveRevision(ctx, p, &next); err != nil {
				return err
			}
			out = p
			return nil
		})
	})
	return out, err
}
func (s *Service) Detail(ctx context.Context, id int64) (*Provider, error) {
	return s.repository.Find(ctx, id)
}
func (s *Service) SetStatus(ctx context.Context, id int64, status int16) error {
	if id <= 0 || (status != 0 && status != 1) {
		return ErrInvalidConfiguration
	}
	return s.mutate(ctx, func(repo Repository) error { return repo.SetStatus(ctx, id, status) })
}
func (s *Service) Models(ctx context.Context, id int64) ([]ProviderModel, error) {
	if id <= 0 {
		return nil, ErrNotFound
	}
	return s.repository.Models(ctx, id)
}
func (s *Service) encryptHeaders(raw []byte) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if s.cipher == nil {
		return nil, ErrInvalidConfiguration
	}
	value, _, err := s.cipher.Seal(string(raw))
	if err != nil {
		return nil, err
	}
	return &value, nil
}
func validateSettings(name, description, baseURL string, headers, hosts []byte, timeout int) error {
	if name == "" || len(name) > 100 || len(description) > 1000 || timeout < 1000 || timeout > 300000 || len(baseURL) > 2048 {
		return ErrInvalidConfiguration
	}
	if err := validURL(baseURL); err != nil {
		return err
	}
	if len(headers) > 8192 || len(hosts) > 8192 {
		return ErrInvalidConfiguration
	}
	if _, err := ai.DecodeJSONArray(hosts); err != nil {
		return ErrInvalidConfiguration
	}
	var hostList []string
	if json.Unmarshal(hosts, &hostList) != nil || len(hostList) > 32 {
		return ErrInvalidConfiguration
	}
	for _, h := range hostList {
		if h == "" || strings.ContainsAny(h, "/\\:@?# \r\n") || net.ParseIP(h) != nil {
			return ErrInvalidConfiguration
		}
	}
	if len(headers) == 0 {
		return nil
	}
	if _, err := ai.DecodeJSONObject(headers); err != nil {
		return ErrInvalidConfiguration
	}
	var h map[string]string
	if json.Unmarshal(headers, &h) != nil || len(h) > 32 {
		return ErrInvalidConfiguration
	}
	for k, v := range h {
		key := http.CanonicalHeaderKey(k)
		switch key {
		case "Host", "Authorization", "Cookie", "Set-Cookie", "Content-Length", "Transfer-Encoding", "Connection", "Proxy-Authorization", "Proxy-Connection", "Upgrade":
			return ErrInvalidConfiguration
		}
		if key == "" || strings.ContainsAny(k+v, "\r\n") {
			return ErrInvalidConfiguration
		}
	}
	return nil
}
func validURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || net.ParseIP(u.Hostname()) != nil || strings.EqualFold(u.Hostname(), "localhost") || strings.ContainsAny(raw, "\r\n \t") {
		return ErrInvalidConfiguration
	}
	return nil
}
