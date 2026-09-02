package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nowdone/internal/service"
)

const userContextKey = "user_id"

// AuthMiddleware verifies the JWT stored in the http-only "session" cookie
// and attaches the authenticated user id to the request context.
func AuthMiddleware(auth *service.AuthService, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie("session")
		if err != nil || cookie == "" {
			respondError(c, http.StatusUnauthorized, "не авторизован")
			c.Abort()
			return
		}

		userID, err := auth.ParseJWT(cookie)
		if err != nil {
			log.Warn("invalid jwt", "error", err)
			respondError(c, http.StatusUnauthorized, "сессия истекла, войдите снова")
			c.Abort()
			return
		}

		c.Set(userContextKey, userID)
		c.Next()
	}
}

func userIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(userContextKey)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// respondError logs the error server-side and returns a Russian-language
// message to the client, per the global error-handling rule.
func respondError(c *gin.Context, status int, userMessage string) {
	c.JSON(status, gin.H{"error": userMessage})
}
