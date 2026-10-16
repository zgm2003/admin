package config

import (
	"admin/server/internal/payment/alipay"
	"admin/server/internal/secretkey"
	cache "admin/server/internal/shared/configCache"
	"context"
	"errors"
	"fmt"
	"time"
)

func (s *Service) Catalog(ctx context.Context) ([]CatalogItem, error) {
	p, err := s.readSnapshot(ctx, "catalog")
	return p.Catalog, err
}
func (s *Service) Credential(ctx context.Context, id, version int64) (BoundCredential, error) {
	if id < 1 || version < 1 {
		return BoundCredential{}, invalid(fmt.Errorf("credential coordinates invalid"))
	}
	p, err := s.readSnapshot(ctx, credentialVariant(id, version))
	if err != nil {
		return BoundCredential{}, err
	}
	return *p.Credential, nil
}

// Credentials resolves one immutable credential version into the provider
// representation used by the checkout/query adapters. The ciphertext never
// leaves this service in a response DTO.
func (s *Service) Credentials(ctx context.Context, id, version int64) (alipay.Credentials, error) {
	bound, err := s.Credential(ctx, id, version)
	if err != nil {
		return alipay.Credentials{}, err
	}
	if s.keys == nil {
		return alipay.Credentials{}, errors.New("payment encryption key missing")
	}
	privateKey, err := secretkey.DecryptPaymentValue(s.keys.PaymentEncryptionKey(), bound.Version.PrivateKeyCiphertext)
	if err != nil {
		return alipay.Credentials{}, err
	}
	return alipay.Credentials{AppID: bound.AppID, SellerID: bound.SellerID, Environment: bound.Environment, PrivateKey: privateKey, AppCertPEM: bound.Version.AppCertPEM, AlipayCertPEM: bound.Version.AlipayCertPEM, RootCertPEM: bound.Version.RootCertPEM, NotifyURL: bound.Version.NotifyURL}, nil
}
func (s *Service) readSnapshot(ctx context.Context, variant string) (snapshot, error) {
	if s.cache == nil || s.cache.client == nil || s.cache.states == nil {
		return snapshot{}, dependency(fmt.Errorf("payment runtime cache missing"))
	}
	// Followers wait at most 500ms; admitted leaders retain cacheFill's 4s I/O budget.
	readCtx, readCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer readCancel()
	for {
		if err := readCtx.Err(); err != nil {
			return snapshot{}, dependency(err)
		}
		state, found, err := s.states.Read(readCtx, configScope)
		repair := !found || errors.Is(err, cache.ErrStateCorrupt)
		if err != nil && !errors.Is(err, cache.ErrStateCorrupt) {
			return snapshot{}, dependency(err)
		}
		if !repair && state.State != cache.StateReady {
			if err := wait(readCtx); err != nil {
				return snapshot{}, dependency(err)
			}
			continue
		}
		if !repair {
			p, hit, err := s.cache.get(readCtx, state.Generation, variant)
			if err != nil && !errors.Is(err, errSnapshotCorrupt) {
				return snapshot{}, dependency(err)
			}
			if err == nil && hit {
				return p, nil
			}
		}
		p, filled, err := s.fillSnapshot(ctx, readCtx, state.Generation, variant, repair)
		if err != nil {
			if errors.Is(err, errGenerationAdvanced) {
				continue
			}
			return snapshot{}, mapError(err)
		}
		if filled {
			return p, nil
		}
	}
}
func (s *Service) fillSnapshot(ctx, admission context.Context, generation int64, variant string, repair bool) (result snapshot, filled bool, resultErr error) {
	lease, err := s.cache.tryFill(admission, generation, variant, repair)
	if err != nil {
		return snapshot{}, false, err
	}
	if lease == nil {
		return snapshot{}, false, wait(admission)
	}
	defer func() { resultErr = errors.Join(resultErr, lease.Release(ctx)) }()
	work, cancel := lease.WorkContext(ctx)
	defer cancel()
	if repair {
		generation, err = s.generations.Current(work, configScope)
		if err != nil {
			return snapshot{}, false, err
		}
		published, err := s.states.Reconcile(work, configScope, generation)
		if err != nil {
			return snapshot{}, false, err
		}
		if published == cache.PublishSkippedInvalidating {
			return snapshot{}, false, errGenerationAdvanced
		}
		if published == cache.PublishAlreadyNewer {
			return snapshot{}, false, errGenerationAdvanced
		}
	}
	if p, hit, err := s.cache.get(work, generation, variant); err != nil && !errors.Is(err, errSnapshotCorrupt) {
		return snapshot{}, false, err
	} else if hit {
		state, found, err := s.states.Read(work, configScope)
		if err != nil {
			return snapshot{}, false, err
		}
		if !found || state.State != cache.StateReady || state.Generation != generation {
			return snapshot{}, false, errGenerationAdvanced
		}
		return p, true, nil
	}
	p := snapshot{SchemaVersion: 1, Generation: generation, Variant: variant}
	if variant == "catalog" {
		p.Catalog, err = s.repository.CatalogFacts(work)
	} else {
		var id, version int64
		if _, err = fmt.Sscanf(variant, "credential:%d:%d", &id, &version); err == nil {
			var b BoundCredential
			b, err = s.repository.CredentialFacts(work, id, version)
			p.Credential = &b
		}
	}
	if err != nil {
		return snapshot{}, false, err
	}
	state, found, err := s.states.Read(work, configScope)
	if err != nil {
		return snapshot{}, false, err
	}
	if !found || state.State != cache.StateReady || state.Generation != generation {
		return snapshot{}, false, errGenerationAdvanced
	}
	if err := s.cache.put(work, p); err != nil {
		return snapshot{}, false, err
	}
	// Confirm publication still belongs to a ready generation before returning.
	state, found, err = s.states.Read(work, configScope)
	if err != nil {
		return snapshot{}, false, err
	}
	if !found || state.State != cache.StateReady || state.Generation != generation {
		return snapshot{}, false, errGenerationAdvanced
	}
	return p, true, nil
}
