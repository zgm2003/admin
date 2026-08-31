package permission

import (
	"sync"
	"time"
)

const defaultLocalSnapshotCacheEntries = 1024

type SnapshotCacheKey struct {
	UserID        int64
	PlatformID    int64
	Platform      string
	PolicyVersion int64
	AccessVersion int64
}

type localSnapshotEntry struct {
	snapshot  CachedSnapshot
	createdAt time.Time
	expiresAt time.Time
}

type LocalSnapshotCache struct {
	mu         sync.Mutex
	entries    map[SnapshotCacheKey]localSnapshotEntry
	maxEntries int
	now        func() time.Time
}

func NewLocalSnapshotCache(maxEntries int) *LocalSnapshotCache {
	if maxEntries <= 0 {
		maxEntries = defaultLocalSnapshotCacheEntries
	}
	return &LocalSnapshotCache{
		entries:    make(map[SnapshotCacheKey]localSnapshotEntry, maxEntries),
		maxEntries: maxEntries,
		now:        time.Now,
	}
}

func (c *LocalSnapshotCache) Read(key SnapshotCacheKey, now time.Time) (CachedSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, found := c.entries[key]
	if !found {
		return CachedSnapshot{}, false
	}
	if !now.Before(entry.expiresAt) {
		delete(c.entries, key)
		return CachedSnapshot{}, false
	}
	return cloneCachedSnapshot(entry.snapshot), true
}

func (c *LocalSnapshotCache) Put(key SnapshotCacheKey, snapshot CachedSnapshot, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	createdAt := c.now()
	if !createdAt.Before(expiresAt) {
		return
	}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}
	c.entries[key] = localSnapshotEntry{
		snapshot:  cloneCachedSnapshot(snapshot),
		createdAt: createdAt,
		expiresAt: expiresAt,
	}
}

func (c *LocalSnapshotCache) evictOldest() {
	var candidate SnapshotCacheKey
	var candidateEntry localSnapshotEntry
	found := false
	for key, entry := range c.entries {
		if !found || entry.createdAt.Before(candidateEntry.createdAt) ||
			(entry.createdAt.Equal(candidateEntry.createdAt) && snapshotCacheKeyLess(key, candidate)) {
			candidate = key
			candidateEntry = entry
			found = true
		}
	}
	if found {
		delete(c.entries, candidate)
	}
}

func snapshotCacheKeyLess(left, right SnapshotCacheKey) bool {
	if left.UserID != right.UserID {
		return left.UserID < right.UserID
	}
	if left.PlatformID != right.PlatformID {
		return left.PlatformID < right.PlatformID
	}
	if left.Platform != right.Platform {
		return left.Platform < right.Platform
	}
	if left.PolicyVersion != right.PolicyVersion {
		return left.PolicyVersion < right.PolicyVersion
	}
	return left.AccessVersion < right.AccessVersion
}

func cloneCachedSnapshot(snapshot CachedSnapshot) CachedSnapshot {
	snapshot.RoleCodes = append([]string(nil), snapshot.RoleCodes...)
	snapshot.MenuTree = cloneMenuTree(snapshot.MenuTree)
	snapshot.PermissionCodes = append([]string(nil), snapshot.PermissionCodes...)
	return snapshot
}
