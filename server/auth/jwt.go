package auth

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
	ErrMissingSecret = errors.New("auth: C2_JWT_SECRET is not configured")
)

// Claims is the payload carried by every C2-DECT Lite token.
//
//	tid  - tenant the token is scoped to
//	role - admin | operator | viewer
type Claims struct {
	OperatorID int64  `json:"operator_id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	TenantID   int64  `json:"tid"`
	TokenType  string `json:"typ"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret         []byte
	accessExpiry   time.Duration
	refreshExpiry  time.Duration
	issuer         string
	audience       string
}

// NewJWTManager builds a manager. The secret is read from C2_JWT_SECRET and
// the process refuses to start without it: a hard coded fallback would let
// anybody forge an operator token on a freshly cloned repository.
func NewJWTManager(accessMinutes, refreshHours int) (*JWTManager, error) {
	secret := os.Getenv("C2_JWT_SECRET")
	if len(secret) < 32 {
		return nil, ErrMissingSecret
	}
	if accessMinutes <= 0 {
		accessMinutes = 30
	}
	if refreshHours <= 0 {
		refreshHours = 12
	}
	return &JWTManager{
		secret:        []byte(secret),
		accessExpiry:  time.Duration(accessMinutes) * time.Minute,
		refreshExpiry: time.Duration(refreshHours) * time.Hour,
		issuer:        "c2-dect-lite",
		audience:      "c2-dect-lite-api",
	}, nil
}

// AccessTTL exposes the access token lifetime so the API can advertise it.
func (m *JWTManager) AccessTTL() time.Duration { return m.accessExpiry }

// RefreshTTL exposes the refresh token lifetime.
func (m *JWTManager) RefreshTTL() time.Duration { return m.refreshExpiry }

// GenerateToken issues a signed access token for an operator.
func (m *JWTManager) GenerateToken(operatorID int64, username, role string, tenantID int64) (string, error) {
	now := time.Now()
	claims := Claims{
		OperatorID: operatorID,
		Username:   username,
		Role:       role,
		TenantID:   tenantID,
		TokenType:  "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiry)),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			Subject:   strconv.FormatInt(operatorID, 10),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidateToken verifies the signature, the issuer, the audience and the
// expiry of an access token.
func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.OperatorID == 0 || claims.Username == "" {
		return nil, ErrInvalidToken
	}
	if claims.TokenType != "" && claims.TokenType != "access" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// NewRefreshID mints the identifier stored with a refresh token.
func NewRefreshID() string { return uuid.New().String() }
