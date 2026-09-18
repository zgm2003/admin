package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	mailtemplate "admin/server/internal/module/message/mail/template"
)

const (
	verifyCodeReadinessSchemaVersion = 1
	verifyCodeReadinessTTLMaximum    = 60
)

var ErrReadinessSnapshotCorrupt = errors.New("mail readiness snapshot is corrupt")

type VerifyCodeReadinessStore interface {
	Current(context.Context, string) (VerifyCodeReadiness, error)
}

type verifyCodeReadinessSnapshot struct {
	SchemaVersion int   `json:"schemaVersion"`
	Generation    int64 `json:"generation"`
	Ready         bool  `json:"ready"`
	TTLMinutes    int   `json:"ttlMinutes"`
}

func validateVerifyCodeReadinessCoordinates(scene string) error {
	if !mailtemplate.IsVerificationScene(scene) {
		return fmt.Errorf("mail verification readiness coordinates are invalid")
	}
	return nil
}

func readinessVariant(scene string) (string, error) {
	if err := validateVerifyCodeReadinessCoordinates(scene); err != nil {
		return "", err
	}
	return "readiness:" + scene, nil
}

func readinessFromSnapshot(snapshot verifyCodeReadinessSnapshot) VerifyCodeReadiness {
	return VerifyCodeReadiness{Ready: snapshot.Ready, TTLMinutes: snapshot.TTLMinutes}
}

func newVerifyCodeReadinessSnapshot(generation int64, readiness VerifyCodeReadiness) verifyCodeReadinessSnapshot {
	ttl := readiness.TTLMinutes
	if !readiness.Ready {
		ttl = 0
	}
	return verifyCodeReadinessSnapshot{
		SchemaVersion: verifyCodeReadinessSchemaVersion,
		Generation:    generation,
		Ready:         readiness.Ready,
		TTLMinutes:    ttl,
	}
}

func encodeVerifyCodeReadinessSnapshot(snapshot verifyCodeReadinessSnapshot) (string, error) {
	if err := validateVerifyCodeReadinessSnapshot(snapshot); err != nil {
		return "", err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode mail verification readiness: %w", err)
	}
	return string(payload), nil
}

func decodeVerifyCodeReadinessSnapshot(raw string) (verifyCodeReadinessSnapshot, error) {
	if err := rejectDuplicateJSONKeys([]byte(raw)); err != nil {
		return verifyCodeReadinessSnapshot{}, corruptReadiness("decode payload: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot verifyCodeReadinessSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return verifyCodeReadinessSnapshot{}, corruptReadiness("decode payload: %v", err)
	}
	if err := ensureReadinessJSONEnd(decoder); err != nil {
		return verifyCodeReadinessSnapshot{}, err
	}
	if err := validateVerifyCodeReadinessSnapshot(snapshot); err != nil {
		return verifyCodeReadinessSnapshot{}, err
	}
	return snapshot, nil
}

func ensureReadinessJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return corruptReadiness("multiple JSON values")
		}
		return corruptReadiness("decode trailing value: %v", err)
	}
	return nil
}

func validateVerifyCodeReadinessSnapshot(snapshot verifyCodeReadinessSnapshot) error {
	if snapshot.SchemaVersion != verifyCodeReadinessSchemaVersion || snapshot.Generation < 1 {
		return corruptReadiness("snapshot coordinates are invalid")
	}
	if snapshot.Ready {
		if snapshot.TTLMinutes < 1 || snapshot.TTLMinutes > verifyCodeReadinessTTLMaximum {
			return corruptReadiness("snapshot TTL is invalid")
		}
	} else if snapshot.TTLMinutes != 0 {
		return corruptReadiness("unready snapshot carries a TTL")
	}
	return nil
}

func corruptReadiness(format string, values ...any) error {
	return fmt.Errorf("%w: %s", ErrReadinessSnapshotCorrupt, fmt.Sprintf(format, values...))
}
