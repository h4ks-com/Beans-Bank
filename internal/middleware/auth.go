package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h4ks-com/beapin/internal/services"
)

type AuthMiddleware struct {
	tokenService *services.TokenService
	testMode     bool
}

func NewAuthMiddleware(tokenService *services.TokenService, testMode bool) *AuthMiddleware {
	return &AuthMiddleware{
		tokenService: tokenService,
		testMode:     testMode,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.testMode {
			username := c.GetHeader("X-Test-Username")
			if username == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Test-Username header required in test mode"})
				c.Abort()
				return
			}
			c.Set("username", username)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := m.tokenService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

func GetUsername(c *gin.Context) string {
	username, exists := c.Get("username")
	if !exists {
		return ""
	}
	return username.(string)
}
