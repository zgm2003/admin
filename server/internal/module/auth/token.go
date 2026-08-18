package auth

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

const (
	AccessTTL = 15 * time.Minute
	jwtIssuer = "admin-api"
)

type JWT struct {
	signingKey []byte
	now        func() time.Time
}

type Claims struct {
	UserID    int64 `json:"uid"`
	SessionID int64 `json:"sid"`
	Version   int64 `json:"ver"`
	jwtlib.RegisteredClaims
}

func NewJWT(signingKey []byte) *JWT {
	return &JWT{
		signingKey: append([]byte(nil), signingKey...),
		now:        time.Now,
	}
}

func (j *JWT) Issue(identity Identity) (string, time.Time, error) {
	if err := validateIdentity(identity); err != nil {
		return "", time.Time{}, err
	}
	now := j.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(AccessTTL)
	claims := Claims{
		UserID:    identity.UserID,
		SessionID: identity.SessionID,
		Version:   identity.Version,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    jwtIssuer,
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(expiresAt),
		},
	}
	raw, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(j.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign Access Token: %w", err)
	}
	return raw, expiresAt, nil
}

func (j *JWT) Parse(raw string) (Identity, error) {
	claims := Claims{}
	parser := jwtlib.NewParser(
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}),
		jwtlib.WithIssuer(jwtIssuer),
		jwtlib.WithExpirationRequired(),
		jwtlib.WithIssuedAt(),
		jwtlib.WithTimeFunc(j.now),
	)
	token, err := parser.ParseWithClaims(raw, &claims, func(token *jwtlib.Token) (any, error) {
		if token.Method != jwtlib.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected JWT signing method %s", token.Method.Alg())
		}
		return j.signingKey, nil
	})
	if err != nil {
		return Identity{}, fmt.Errorf("parse Access Token: %w", err)
	}
	if !token.Valid {
		return Identity{}, fmt.Errorf("Access Token is invalid")
	}
	if claims.IssuedAt == nil || claims.NotBefore == nil || claims.ExpiresAt == nil {
		return Identity{}, fmt.Errorf("Access Token requires issued-at, not-before, and expiry claims")
	}
	identity := Identity{UserID: claims.UserID, SessionID: claims.SessionID, Version: claims.Version}
	if err := validateIdentity(identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func validateIdentity(identity Identity) error {
	if identity.UserID <= 0 || identity.SessionID <= 0 || identity.Version <= 0 {
		return fmt.Errorf("identity user, session, and version must be positive")
	}
	return nil
}
