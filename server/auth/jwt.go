package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type JWTManager struct {
	secret     []byte
	expiryTime time.Duration
}

type Claims struct {
	OperatorID int64  `json:"operator_id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

func NewJWTManager(secret string, expiryMinutes int) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		expiryTime: time.Duration(expiryMinutes) * time.Minute,
	}
}

func (m *JWTManager) GenerateToken(operatorID int64, username, role string) (string, error) {
	claims := Claims{
		OperatorID: operatorID,
		Username:   username,
		Role:       role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expiryTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "c2-dect",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
