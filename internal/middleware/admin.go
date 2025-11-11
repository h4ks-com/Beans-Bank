package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AdminMiddleware struct {
	adminUsers []string
}

func NewAdminMiddleware(adminUsers []string) *AdminMiddleware {
	return &AdminMiddleware{
		adminUsers: adminUsers,
	}
}

func (m *AdminMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := GetUsername(c)
		if username == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		isAdmin := false
		for _, admin := range m.adminUsers {
			if admin == username {
				isAdmin = true
				break
			}
		}

		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}
