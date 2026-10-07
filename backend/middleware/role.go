package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RoleMiddleware(roles ...string) gin.HandlerFunc {

	return func(c *gin.Context) {

		userRoleRaw, exists := c.Get("role_name")

		if !exists {

			c.JSON(
				http.StatusForbidden,
				gin.H{
					"error": "Role not found",
				},
			)

			c.Abort()
			return
		}

		userRole, ok := userRoleRaw.(string)
		allowed := false
		if ok {
			for _, r := range roles {
				if strings.EqualFold(userRole, r) {
					allowed = true
					break
				}
			}
		}

		if !allowed {

			c.JSON(
				http.StatusForbidden,
				gin.H{
					"error": "Permission denied",
				},
			)

			c.Abort()
			return
		}

		c.Next()

	}

}
