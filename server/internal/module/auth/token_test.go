package auth

import (
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestJWTIssueAndParseRoundTrip(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 10, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	codec := NewJWT(key)
	codec.now = func() time.Time { return fixedNow }
	key[0] = 'x'
	want := Identity{UserID: 11, SessionID: 22, Version: 3}

	raw, expiresAt, err := codec.Issue(want)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || !expiresAt.Equal(fixedNow.Add(AccessTTL)) {
		t.Fatalf("Issue() = token %q expires %v", raw, expiresAt)
	}
	got, err := codec.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Parse() = %+v, want %+v", got, want)
	}
}

func TestJWTRejectsWrongAlgorithmAndSecret(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 10, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	codec := NewJWT(key)
	codec.now = func() time.Time { return fixedNow }
	claims := validClaims(fixedNow)

	wrongAlgorithm, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS512, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.Parse(wrongAlgorithm); err == nil {
		t.Fatal("HS512 token was accepted")
	}

	validToken, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	wrongKeyCodec := NewJWT([]byte(strings.Repeat("z", 32)))
	wrongKeyCodec.now = func() time.Time { return fixedNow }
	if _, err := wrongKeyCodec.Parse(validToken); err == nil {
		t.Fatal("token signed by another key was accepted")
	}
}

func TestJWTRejectsInvalidTimeClaims(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 10, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	codec := NewJWT(key)
	codec.now = func() time.Time { return fixedNow }

	expired := validClaims(fixedNow)
	expired.ExpiresAt = jwtlib.NewNumericDate(fixedNow.Add(-time.Second))
	assertJWTRejected(t, codec, key, expired)

	future := validClaims(fixedNow)
	future.NotBefore = jwtlib.NewNumericDate(fixedNow.Add(time.Minute))
	assertJWTRejected(t, codec, key, future)

	missingTimes := validClaims(fixedNow)
	missingTimes.IssuedAt = nil
	missingTimes.NotBefore = nil
	missingTimes.ExpiresAt = nil
	assertJWTRejected(t, codec, key, missingTimes)
}

func TestJWTRejectsMalformedClaims(t *testing.T) {
	fixedNow := time.Date(2026, time.August, 17, 10, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	codec := NewJWT(key)
	codec.now = func() time.Time { return fixedNow }

	for _, mutate := range []func(*Claims){
		func(claims *Claims) { claims.UserID = 0 },
		func(claims *Claims) { claims.SessionID = -1 },
		func(claims *Claims) { claims.Version = 0 },
		func(claims *Claims) { claims.Issuer = "another-service" },
	} {
		claims := validClaims(fixedNow)
		mutate(&claims)
		assertJWTRejected(t, codec, key, claims)
	}

	if _, _, err := codec.Issue(Identity{}); err == nil {
		t.Fatal("Issue accepted zero identity")
	}
}

func validClaims(now time.Time) Claims {
	return Claims{
		UserID:    1,
		SessionID: 2,
		Version:   1,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    jwtIssuer,
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(AccessTTL)),
		},
	}
}

func assertJWTRejected(t *testing.T, codec *JWT, key []byte, claims Claims) {
	t.Helper()
	raw, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.Parse(raw); err == nil {
		t.Fatalf("claims %+v were accepted", claims)
	}
}
