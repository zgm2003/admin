package provider

import (
	ai "admin/server/internal/module/ai"
	configcache "admin/server/internal/shared/configCache"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

type ModelReader interface {
	List(context.Context, string, string) ([]OpenAIModel, error)
}
type CredentialOpener interface {
	Open(string, string) (string, error)
}
type CatalogPreview struct {
	ProviderID        int64         `json:"providerId,string"`
	ProviderVersionID int64         `json:"providerVersionId,string"`
	Digest            string        `json:"digest"`
	Added             []OpenAIModel `json:"added"`
	Existing          []OpenAIModel `json:"existing"`
	Models            []OpenAIModel `json:"-"`
}
type CatalogSelection struct {
	UpstreamModelID string          `json:"upstreamModelId"`
	Name            string          `json:"name"`
	OfficialModelID *int64          `json:"officialModelId,omitempty,string"`
	Operations      []string        `json:"operations"`
	Limits          json.RawMessage `json:"limits"`
}
type CatalogService struct {
	providers Repository
	cipher    CredentialOpener
	source    ModelReader
	runtime   RuntimeCoordinator
}

func NewCatalogService(providers Repository, cipher CredentialOpener, source ModelReader) *CatalogService {
	return &CatalogService{providers: providers, cipher: cipher, source: source}
}
func (s *CatalogService) SetRuntimeCoordinator(runtime RuntimeCoordinator) { s.runtime = runtime }

var ErrCatalogChanged = errors.New("AI provider model catalogue changed; preview again")

func (s *CatalogService) Preview(ctx context.Context, providerID int64) (*CatalogPreview, error) {
	if s.source == nil || s.cipher == nil {
		return nil, ErrInvalidConfiguration
	}
	version, err := s.providers.CurrentVersion(ctx, providerID)
	if err != nil {
		return nil, err
	}
	key, err := s.cipher.Open(version.APIKeyCiphertext, version.EncryptionKeyID)
	if err != nil {
		return nil, err
	}
	var items []OpenAIModel
	if version.HeadersCiphertext != nil {
		raw, err := s.cipher.Open(*version.HeadersCiphertext, version.EncryptionKeyID)
		if err != nil {
			return nil, err
		}
		var headers map[string]string
		if err := json.Unmarshal([]byte(raw), &headers); err != nil {
			return nil, ErrInvalidConfiguration
		}
		reader, ok := s.source.(interface {
			ListWithHeaders(context.Context, string, string, map[string]string) ([]OpenAIModel, error)
		})
		if !ok {
			return nil, errors.New("AI model source cannot apply configured headers")
		}
		items, err = reader.ListWithHeaders(ctx, version.BaseURL, key, headers)
	} else {
		items, err = s.source.List(ctx, version.BaseURL, key)
	}
	if err != nil {
		return nil, err
	}
	models, err := s.providers.Models(ctx, providerID)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]struct{}, len(models))
	for _, m := range models {
		existing[m.UpstreamModelID] = struct{}{}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	p := &CatalogPreview{ProviderID: providerID, ProviderVersionID: version.ID, Models: items, Added: []OpenAIModel{}, Existing: []OpenAIModel{}}
	for _, item := range items {
		if _, ok := existing[item.ID]; ok {
			p.Existing = append(p.Existing, item)
		} else {
			p.Added = append(p.Added, item)
		}
	}
	raw, _ := json.Marshal(struct {
		ProviderID, VersionID int64
		Models                []OpenAIModel
	}{providerID, version.ID, items})
	sum := sha256.Sum256(raw)
	p.Digest = hex.EncodeToString(sum[:])
	return p, nil
}

// Confirm re-fetches catalogue identity, then atomically inserts explicitly configured new models.
// GET /models has no capability contract, so no operation, mapping or price is invented.
func (s *CatalogService) Confirm(ctx context.Context, providerID int64, digest string, selections []CatalogSelection) error {
	if digest == "" || len(selections) == 0 || len(selections) > 1000 {
		return ErrInvalidConfiguration
	}
	preview, err := s.Preview(ctx, providerID)
	if err != nil {
		return err
	}
	if preview.Digest != digest {
		return ErrCatalogChanged
	}
	added := make(map[string]struct{}, len(preview.Added))
	for _, m := range preview.Added {
		added[m.ID] = struct{}{}
	}
	seen := map[string]struct{}{}
	models := make([]ProviderModel, 0, len(selections))
	for _, selection := range selections {
		if _, ok := added[selection.UpstreamModelID]; !ok {
			return ErrCatalogChanged
		}
		if _, ok := seen[selection.UpstreamModelID]; ok {
			return ErrInvalidConfiguration
		}
		seen[selection.UpstreamModelID] = struct{}{}
		if selection.Name == "" || len(selection.Name) > 100 || len(selection.Operations) == 0 || len(selection.Operations) > 6 || len(selection.Limits) > 65536 {
			return ErrInvalidConfiguration
		}
		ops := map[string]struct{}{}
		for _, op := range selection.Operations {
			if !ai.Operation(op).Valid() {
				return ErrInvalidConfiguration
			}
			if _, ok := ops[op]; ok {
				return ErrInvalidConfiguration
			}
			ops[op] = struct{}{}
		}
		if _, err := ai.DecodeJSONObject(selection.Limits); err != nil {
			return ErrInvalidConfiguration
		}
		models = append(models, ProviderModel{ProviderID: providerID, UpstreamModelID: selection.UpstreamModelID, Name: selection.Name, OfficialModelID: selection.OfficialModelID, Operations: selection.Operations, Limits: selection.Limits, Status: 0})
	}
	if s.runtime == nil {
		return errors.New("AI runtime mutation coordinator unavailable")
	}
	return s.runtime.Mutate(ctx, func(ctx context.Context, base int64) (configcache.MutationResult, error) {
		return s.providers.Mutate(ctx, base, func(tx Repository) error {
			return tx.WithProvider(ctx, providerID, func(repo Repository) error {
				current, err := repo.CurrentVersion(ctx, providerID)
				if err != nil {
					return err
				}
				if current.ID != preview.ProviderVersionID {
					return ErrCatalogChanged
				}
				existing, err := repo.Models(ctx, providerID)
				if err != nil {
					return err
				}
				for _, m := range existing {
					if _, ok := seen[m.UpstreamModelID]; ok {
						return ErrCatalogChanged
					}
				}
				return repo.SaveCatalog(ctx, providerID, models)
			})
		})
	})
}
