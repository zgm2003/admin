package authstate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	authclient "admin/server/internal/module/auth/client"
	projectredis "admin/server/internal/redis"
)

const SessionSnapshotSchemaVersion = 1

type SessionSnapshot struct {
	SchemaVersion      int       `json:"schemaVersion"`
	UserID             int64     `json:"userId"`
	SessionID          int64     `json:"sessionId"`
	Platform           string    `json:"platform"`
	SessionVersion     int64     `json:"sessionVersion"`
	PolicyVersion      int64     `json:"policyVersion"`
	UserGeneration     string    `json:"userGeneration"`
	SessionsGeneration string    `json:"sessionsGeneration"`
	DeviceID           string    `json:"deviceId"`
	ClientIP           string    `json:"clientIp"`
	RefreshExpiresAt   time.Time `json:"refreshExpiresAt"`
	Revoked            bool      `json:"revoked"`
}

type SessionCache struct {
	redis *projectredis.Client
}

type SessionReference struct {
	Platform  string
	SessionID int64
}

func NewSessionCache(redis *projectredis.Client) *SessionCache {
	return &SessionCache{redis: redis}
}

func SessionKey(platform string, sessionID int64) string {
	return "auth:session:" + platform + ":" + strconv.FormatInt(sessionID, 10)
}

func (c *SessionCache) Read(ctx context.Context, platform string, sessionID int64) (SessionSnapshot, bool, error) {
	raw, found, err := c.redis.GetString(ctx, SessionKey(platform, sessionID))
	if err != nil || !found {
		return SessionSnapshot{}, found, err
	}
	var snapshot SessionSnapshot
	if err := decodeSessionSnapshot(raw, &snapshot); err != nil {
		return SessionSnapshot{}, true, err
	}
	if err := validateSessionSnapshot(platform, sessionID, snapshot); err != nil {
		return SessionSnapshot{}, true, err
	}
	return snapshot, true, nil
}

func (c *SessionCache) PublishIfCurrent(ctx context.Context, snapshot SessionSnapshot, ttl time.Duration) (bool, error) {
	if err := validateSessionSnapshot(snapshot.Platform, snapshot.SessionID, snapshot); err != nil {
		return false, err
	}
	if ttl <= 0 {
		return false, fmt.Errorf("session snapshot TTL must be positive")
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return false, fmt.Errorf("encode session snapshot: %w", err)
	}
	result, err := c.redis.EvalString(ctx, publishSessionSnapshotScript, []string{
		SessionKey(snapshot.Platform, snapshot.SessionID),
		UserStateKey(snapshot.UserID),
		SessionsStateKey(snapshot.Platform, snapshot.UserID),
	}, string(payload), snapshot.UserGeneration, snapshot.SessionsGeneration, ttl.Milliseconds())
	if err != nil {
		return false, err
	}
	switch result {
	case "published":
		return true, nil
	case "changed", "missing", "updating":
		return false, nil
	default:
		return false, fmt.Errorf("publish session snapshot returned %q", result)
	}
}

func (c *SessionCache) Delete(ctx context.Context, platform string, sessionID int64) error {
	return c.redis.Delete(ctx, SessionKey(platform, sessionID))
}

func (c *SessionCache) DeleteMany(ctx context.Context, sessions []SessionReference) error {
	keys := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if session.SessionID < 1 || authclient.ValidatePlatform(session.Platform) != nil {
			return fmt.Errorf("session cache delete identity is invalid")
		}
		keys = append(keys, SessionKey(session.Platform, session.SessionID))
	}
	return c.redis.DeleteMany(ctx, keys)
}

func CleanupLegacySessionPointers(ctx context.Context, redis *projectredis.Client) error {
	return redis.ScanDelete(ctx, "auth:current-session:*")
}

func validateSessionSnapshot(expectedPlatform string, expectedSessionID int64, snapshot SessionSnapshot) error {
	if snapshot.SchemaVersion != SessionSnapshotSchemaVersion || snapshot.UserID < 1 || snapshot.SessionID != expectedSessionID ||
		snapshot.SessionVersion < 1 || snapshot.PolicyVersion < 1 || snapshot.UserGeneration == "" || snapshot.SessionsGeneration == "" {
		return fmt.Errorf("session snapshot identity is invalid")
	}
	if snapshot.Platform != expectedPlatform || authclient.ValidatePlatform(snapshot.Platform) != nil {
		return fmt.Errorf("session snapshot platform is invalid")
	}
	if authclient.ValidateDeviceID(snapshot.DeviceID) != nil || snapshot.ClientIP == "" || len(snapshot.ClientIP) > 64 {
		return fmt.Errorf("session snapshot client metadata is invalid")
	}
	if snapshot.RefreshExpiresAt.IsZero() || snapshot.RefreshExpiresAt.Location() != time.UTC {
		return fmt.Errorf("session snapshot refresh expiry is invalid")
	}
	return nil
}

func decodeSessionSnapshot(raw string, target *SessionSnapshot) error {
	expected := []string{
		"schemaVersion", "userId", "sessionId", "platform", "sessionVersion", "policyVersion",
		"userGeneration", "sessionsGeneration", "deviceId", "clientIp", "refreshExpiresAt", "revoked",
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return fmt.Errorf("decode session snapshot fields: %w", err)
	}
	if len(fields) != len(expected) {
		return fmt.Errorf("session snapshot JSON field set is invalid")
	}
	for _, key := range expected {
		if _, ok := fields[key]; !ok {
			return fmt.Errorf("session snapshot field %q is missing", key)
		}
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode session snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("session snapshot has trailing data")
	}
	return nil
}

const publishSessionSnapshotScript = `
local userState = redis.call('GET', KEYS[2])
local sessionsState = redis.call('GET', KEYS[3])
if not userState or not sessionsState then return 'missing' end
local userDecoded = cjson.decode(userState)
local sessionsDecoded = cjson.decode(sessionsState)
if userDecoded.state == 'invalidating' or sessionsDecoded.state == 'invalidating' then return 'updating' end
if userDecoded.state ~= 'ready' or sessionsDecoded.state ~= 'ready' then return 'changed' end
if userDecoded.generation ~= ARGV[2] or sessionsDecoded.generation ~= ARGV[3] then return 'changed' end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[4])
return 'published'
`
