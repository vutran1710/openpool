package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	PublicIDKey contextKey = "public_id"
)

type AuthMiddleware struct {
	secret []byte
}

func NewAuth(secret string) *AuthMiddleware {
	return &AuthMiddleware{secret: []byte(secret)}
}

func (a *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		claims, err := a.validateToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, PublicIDKey, claims.PublicID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type Claims struct {
	UserID   uuid.UUID `json:"sub"`
	PublicID string    `json:"pid"`
	jwt.RegisteredClaims
}

func (a *AuthMiddleware) validateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return a.secret, nil
	})
	return claims, err
}

func (a *AuthMiddleware) GenerateToken(userID uuid.UUID, publicID string) (string, error) {
	claims := Claims{
		UserID:   userID,
		PublicID: publicID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "dating-dev",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

func extractToken(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(UserIDKey).(uuid.UUID)
	return id
}

func PublicIDFromContext(ctx context.Context) string {
	pid, _ := ctx.Value(PublicIDKey).(string)
	return pid
}
