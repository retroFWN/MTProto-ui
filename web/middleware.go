package web

import (
	"net/http"
	"strings"

	"mtproxy-panel/auth"
	"mtproxy-panel/database"

	"github.com/gin-gonic/gin"
)

const userKey = "currentUser"

func extractToken(c *gin.Context) string {
	// 1. Cookie
	if token, err := c.Cookie("access_token"); err == nil && token != "" {
		return token
	}
	// 2. Authorization header
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}
	return ""
}

func authenticate(c *gin.Context) *database.User {
	token := extractToken(c)
	if token == "" {
		return nil
	}

	username, err := auth.ValidateToken(token)
	if err != nil || username == "" {
		return nil
	}

	var user database.User
	if database.DB.Where("username = ?", username).First(&user).Error != nil {
		return nil
	}
	return &user
}

// PageAuth redirects to login page if not authenticated.
func PageAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authenticate(c)
		if user == nil {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}
		c.Set(userKey, user)
		c.Next()
	}
}

// APIAuth returns 401 JSON if not authenticated.
func APIAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authenticate(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Not authenticated"})
			c.Abort()
			return
		}
		c.Set(userKey, user)
		c.Next()
	}
}

// BotAuth validates X-Bot-Token header against the panel's secret key.
func BotAuth(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Bot-Token")
		if token == "" || token != secretKey {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Invalid bot token"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) *database.User {
	u, _ := c.Get(userKey)
	if user, ok := u.(*database.User); ok {
		return user
	}
	return nil
}
