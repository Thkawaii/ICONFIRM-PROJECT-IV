package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var JwtKey = jwtKeyFromEnv()

func jwtKeyFromEnv() []byte {
	if v := os.Getenv("JWT_SECRET"); v != "" {
		return []byte(v)
	}

	log.Println("[WARNING] ไม่ได้ตั้ง env JWT_SECRET — ใช้ default key (ใช้ได้แค่ dev เท่านั้น ห้ามใช้ production)")
	return []byte("iconfirm-secret-key")
}

func AuthMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})

			c.Abort()
			return
		}

		tokenString := strings.Replace(
			authHeader,
			"Bearer ",
			"",
			1,
		)

		token, err := jwt.Parse(
			tokenString,

			func(token *jwt.Token) (interface{}, error) {

				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {

					return nil, jwt.ErrSignatureInvalid

				}

				return JwtKey, nil
			},
		)

		if err != nil || !token.Valid {

			c.JSON(http.StatusUnauthorized, gin.H{

				"error": "Invalid token",
			})

			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {

			c.Set(
				"user_id",
				claims["id"],
			)

			c.Set(
				"role_name",
				claims["role_name"],
			)

			c.Set(
				"username",
				claims["username"],
			)

		}

		c.Next()

	}
}
