package config

import (
	"admin/server/internal/module/payment"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheFill"
	cache "admin/server/internal/shared/configCache"
	"admin/server/internal/shared/yesno"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"
)

const snapshotTTL = 5 * time.Minute

var errSnapshotCorrupt = errors.New("payment configuration snapshot corrupt")
var errGenerationAdvanced = errors.New("payment generation advanced")

type CatalogItem struct {
	ID             int64               `json:"id"`
	Name           string              `json:"name"`
	Provider       payment.Provider    `json:"provider"`
	Environment    payment.Environment `json:"environment"`
	AppID          string              `json:"appId"`
	SellerID       string              `json:"sellerId"`
	CurrentVersion int64               `json:"currentVersion"`
	SortOrder      int32               `json:"sortOrder"`
	Status      yesno.Value         `json:"status"`
}
type BoundCredential struct {
	ConfigID    int64               `json:"configId"`
	Provider    payment.Provider    `json:"provider"`
	Environment payment.Environment `json:"environment"`
	AppID       string              `json:"appId"`
	SellerID    string              `json:"sellerId"`
	Version     CredentialVersion   `json:"version"`
}

// Cache transport is independent of the GORM persistence model.
type CredentialVersion struct {
	ConfigID             int64      `json:"configId"`
	Version              int64      `json:"version"`
	PrivateKeyCiphertext string     `json:"privateKeyCiphertext"`
	AppCertPEM           string     `json:"appCertPEM"`
	AlipayCertPEM        string     `json:"alipayCertPEM"`
	RootCertPEM          string     `json:"rootCertPEM"`
	NotifyURL            string     `json:"notifyUrl"`
	PayMethods           PayMethods `json:"payMethods"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}
type snapshot struct {
	SchemaVersion int              `json:"schemaVersion"`
	Generation    int64            `json:"generation"`
	Variant       string           `json:"variant"`
	Catalog       []CatalogItem    `json:"catalog"`
	Credential    *BoundCredential `json:"credential"`
}
type Cache struct {
	client *projectredis.Client
	states *cache.Store
}

func NewCache(client *projectredis.Client, states *cache.Store) *Cache { return &Cache{client, states} }
func credentialVariant(id, version int64) string                       { return fmt.Sprintf("credential:%d:%d", id, version) }
func (c *Cache) get(ctx context.Context, generation int64, variant string) (snapshot, bool, error) {
	key, err := cache.SnapshotKey(configScope, generation, variant)
	if err != nil {
		return snapshot{}, false, err
	}
	raw, found, err := c.client.GetString(ctx, key)
	if err != nil || !found {
		return snapshot{}, false, err
	}
	var p snapshot
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > MaxSnapshotBytes || decoder.Decode(&p) != nil || decoder.Decode(&struct{}{}) != io.EOF || p.SchemaVersion != 1 || p.Generation != generation || p.Variant != variant {
		return snapshot{}, false, errSnapshotCorrupt
	}
	if err := validateSnapshot(p); err != nil {
		return snapshot{}, false, errSnapshotCorrupt
	}
	// Only our complete canonical transport is a hit. This also rejects duplicate,
	// missing, null-for-scalar, wrong-case and unknown fields at every nesting level.
	canonical, err := json.Marshal(p)
	var compact bytes.Buffer
	if err != nil || json.Compact(&compact, []byte(raw)) != nil || !bytes.Equal(canonical, compact.Bytes()) {
		return snapshot{}, false, errSnapshotCorrupt
	}
	return p, true, nil
}
func validateSnapshot(p snapshot) error {
	if p.Variant == "catalog" {
		if p.Catalog == nil || p.Credential != nil {
			return errSnapshotCorrupt
		}
		ids := map[int64]bool{}
		for _, m := range p.Catalog {
			if m.ID < 1 || ids[m.ID] || validateMetadata(MetadataInput{m.Name, m.SortOrder}) != nil || !identityPattern.MatchString(m.AppID) || !identityPattern.MatchString(m.SellerID) || !m.Provider.Supported() || !m.Environment.Valid() || !yesno.IsValid(m.Status) || m.CurrentVersion < 1 {
				return errSnapshotCorrupt
			}
			ids[m.ID] = true
		}
		return nil
	}
	b := p.Credential
	if p.Catalog != nil || b == nil || b.ConfigID < 1 || b.ConfigID != b.Version.ConfigID || b.Version.Version < 1 || p.Variant != credentialVariant(b.ConfigID, b.Version.Version) || !identityPattern.MatchString(b.AppID) || !identityPattern.MatchString(b.SellerID) || !b.Provider.Supported() || !b.Environment.Valid() || b.Version.PrivateKeyCiphertext == "" || b.Version.AppCertPEM == "" || b.Version.AlipayCertPEM == "" || b.Version.RootCertPEM == "" || b.Version.CreatedAt.IsZero() || !b.Version.CreatedAt.Equal(b.Version.UpdatedAt) {
		return errSnapshotCorrupt
	}
	methods, err := payment.NormalizePayMethods([]payment.PayMethod(b.Version.PayMethods))
	if err != nil || !reflect.DeepEqual(PayMethods(methods), b.Version.PayMethods) {
		return errSnapshotCorrupt
	}
	if _, err := notifyTemplate(b.Version.NotifyURL, b.ConfigID, b.Version.Version); err != nil {
		return errSnapshotCorrupt
	}
	return nil
}
func (c *Cache) put(ctx context.Context, p snapshot) error {
	if err := validateSnapshot(p); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if len(raw) > MaxSnapshotBytes {
		return fmt.Errorf("payment snapshot size limit exceeded")
	}
	key, err := cache.SnapshotKey(configScope, p.Generation, p.Variant)
	if err != nil {
		return err
	}
	return c.client.SetString(ctx, key, string(raw), snapshotTTL)
}
func (c *Cache) tryFill(ctx context.Context, generation int64, variant string, repair bool) (*cachefill.Lease, error) {
	target := "repair"
	if !repair {
		var err error
		target, err = cache.FillKey(configScope, generation, variant)
		if err != nil {
			return nil, err
		}
	}
	return cachefill.Try(ctx, c.client.UniversalClient(), cache.StateKey(configScope), target)
}
