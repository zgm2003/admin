package permission

import (
	"testing"
	"time"
)

func TestLocalSnapshotCacheReturnsDeepCopies(t *testing.T) {
	cache := NewLocalSnapshotCache(2)
	now := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	key := localCacheKey(1)
	cache.Put(key, localCachedSnapshot(1), now.Add(time.Minute))

	first, found := cache.Read(key, now)
	if !found {
		t.Fatal("cache read missed")
	}
	first.RoleCodes[0] = "changed-role"
	first.PermissionCodes[0] = "changed-permission"
	first.MenuTree[0].Children[0].Code = "changed-menu"

	second, found := cache.Read(key, now)
	if !found {
		t.Fatal("second cache read missed")
	}
	if second.RoleCodes[0] != "role" || second.PermissionCodes[0] != "account:user:list" || second.MenuTree[0].Children[0].Code != "account:user:list" {
		t.Fatalf("stored snapshot was mutated: %+v", second)
	}
}

func TestLocalSnapshotCacheExpiresAtBoundary(t *testing.T) {
	cache := NewLocalSnapshotCache(2)
	now := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	key := localCacheKey(1)
	expiresAt := now.Add(time.Minute)
	cache.Put(key, localCachedSnapshot(1), expiresAt)

	if _, found := cache.Read(key, expiresAt); found {
		t.Fatal("entry remained readable at expiresAt")
	}
	if len(cache.entries) != 0 {
		t.Fatalf("expired entry count = %d, want 0", len(cache.entries))
	}
}

func TestLocalSnapshotCacheEvictsOldestEntry(t *testing.T) {
	cache := NewLocalSnapshotCache(2)
	now := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	cache.Put(localCacheKey(1), localCachedSnapshot(1), now.Add(time.Hour))
	now = now.Add(time.Second)
	cache.Put(localCacheKey(2), localCachedSnapshot(2), now.Add(time.Hour))
	now = now.Add(time.Second)
	cache.Put(localCacheKey(3), localCachedSnapshot(3), now.Add(time.Hour))

	if _, found := cache.Read(localCacheKey(1), now); found {
		t.Fatal("oldest entry was not evicted")
	}
	if _, found := cache.Read(localCacheKey(2), now); !found {
		t.Fatal("second entry was evicted")
	}
	if _, found := cache.Read(localCacheKey(3), now); !found {
		t.Fatal("newest entry was evicted")
	}
}

func TestLocalSnapshotCacheBreaksEvictionTiesByKey(t *testing.T) {
	cache := NewLocalSnapshotCache(2)
	now := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	cache.Put(localCacheKey(2), localCachedSnapshot(2), now.Add(time.Hour))
	cache.Put(localCacheKey(1), localCachedSnapshot(1), now.Add(time.Hour))
	cache.Put(localCacheKey(3), localCachedSnapshot(3), now.Add(time.Hour))

	if _, found := cache.Read(localCacheKey(1), now); found {
		t.Fatal("lexicographically first key was not evicted for a createdAt tie")
	}
	if _, found := cache.Read(localCacheKey(2), now); !found {
		t.Fatal("larger tied key was evicted")
	}
}

func TestLocalSnapshotCacheUsesCompleteKey(t *testing.T) {
	cache := NewLocalSnapshotCache(8)
	now := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	key := localCacheKey(1)
	cache.Put(key, localCachedSnapshot(1), now.Add(time.Hour))

	variants := []SnapshotCacheKey{
		{UserID: 8, PlatformID: key.PlatformID, Platform: key.Platform, PolicyVersion: key.PolicyVersion, AccessVersion: key.AccessVersion},
		{UserID: key.UserID, PlatformID: 2, Platform: key.Platform, PolicyVersion: key.PolicyVersion, AccessVersion: key.AccessVersion},
		{UserID: key.UserID, PlatformID: key.PlatformID, Platform: "canvas", PolicyVersion: key.PolicyVersion, AccessVersion: key.AccessVersion},
		{UserID: key.UserID, PlatformID: key.PlatformID, Platform: key.Platform, PolicyVersion: 5, AccessVersion: key.AccessVersion},
		{UserID: key.UserID, PlatformID: key.PlatformID, Platform: key.Platform, PolicyVersion: key.PolicyVersion, AccessVersion: 2},
	}
	for _, variant := range variants {
		if _, found := cache.Read(variant, now); found {
			t.Fatalf("cache hit for different key: %+v", variant)
		}
	}
}

func localCacheKey(version int64) SnapshotCacheKey {
	return SnapshotCacheKey{UserID: 7, PlatformID: 1, Platform: "admin", PolicyVersion: 4, AccessVersion: version}
}

func localCachedSnapshot(version int64) CachedSnapshot {
	path := "/account/users"
	componentPath := "account/users"
	return CachedSnapshot{
		SchemaVersion: accessSnapshotSchemaVersion,
		UserID:        7,
		PlatformID:    1,
		Platform:      "admin",
		PolicyVersion: 4,
		Version:       version,
		RoleCodes:     []string{"role"},
		MenuTree: []MenuNode{{
			Code: "account", MenuType: MenuDirectory, I18nKey: "navigation.account", Children: []MenuNode{{
				Code: "account:user:list", MenuType: MenuPage, Path: &path, ComponentPath: &componentPath,
				I18nKey: "navigation.accountUsers", Children: []MenuNode{},
			}},
		}},
		PermissionCodes: []string{"account:user:list"},
	}
}
