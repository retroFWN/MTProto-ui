package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"mtproxy-panel/auth"
	"mtproxy-panel/database"

	"github.com/gin-gonic/gin"
)

const userKey = "currentUser"

// ── Rate limiter for login ──────────────────────────────────────────────

type loginAttempt struct {
	count    int
	blockedUntil time.Time
}

var (
	loginAttempts = map[string]*loginAttempt{}
	loginMu       sync.Mutex
)

const (
	maxLoginAttempts = 5
	loginBlockDuration = 5 * time.Minute
)

// LoginRateLimit blocks IPs after too many failed login attempts.
func LoginRateLimit() gin.HandlerFunc {
	// Cleanup old entries every 10 minutes
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			loginMu.Lock()
			now := time.Now()
			for ip, a := range loginAttempts {
				if now.After(a.blockedUntil) && a.count == 0 {
					delete(loginAttempts, ip)
				}
			}
			loginMu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		loginMu.Lock()
		a, exists := loginAttempts[ip]
		if exists && time.Now().Before(a.blockedUntil) {
			remaining := int(time.Until(a.blockedUntil).Seconds())
			loginMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"detail": "Too many login attempts. Try again later.",
				"retry_after": remaining,
			})
			c.Abort()
			return
		}
		loginMu.Unlock()

		c.Next()

		// After handler: track failed attempts
		if c.Writer.Status() == http.StatusUnauthorized {
			loginMu.Lock()
			if !exists {
				a = &loginAttempt{}
				loginAttempts[ip] = a
			}
			a.count++
			if a.count >= maxLoginAttempts {
				a.blockedUntil = time.Now().Add(loginBlockDuration)
				a.count = 0
			}
			loginMu.Unlock()
		} else if c.Writer.Status() == http.StatusOK {
			// Successful login — reset counter
			loginMu.Lock()
			delete(loginAttempts, ip)
			loginMu.Unlock()
		}
	}
}

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

// ── CSRF protection (double-submit cookie) ──────────────────────────────

func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip safe methods
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			// Set CSRF cookie if not present (readable by JS — not httpOnly)
			if _, err := c.Cookie("csrf_token"); err != nil {
				b := make([]byte, 16)
				rand.Read(b)
				token := hex.EncodeToString(b)
				c.SetCookie("csrf_token", token, 3600*24, "/", "", false, false)
			}
			c.Next()
			return
		}

		// For state-changing methods: verify X-CSRF-Token header matches cookie
		cookie, err := c.Cookie("csrf_token")
		if err != nil || cookie == "" {
			c.JSON(http.StatusForbidden, gin.H{"detail": "CSRF token missing"})
			c.Abort()
			return
		}

		header := c.GetHeader("X-CSRF-Token")
		if header == "" || header != cookie {
			c.JSON(http.StatusForbidden, gin.H{"detail": "CSRF token invalid"})
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
