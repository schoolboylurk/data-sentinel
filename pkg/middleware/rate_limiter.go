package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var userTimestamps = make(map[string][]time.Time)

const maxRequests = 5

func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := sessions.Default(c)
		kid := sess.Get("kid")
		if kid == nil {
			// not our route or not logged in
			c.Next()
			return
		}
		now := time.Now()
		times := userTimestamps[kid.(string)]
		// keep only the last 60s
		var recent []time.Time
		for _, t := range times {
			if now.Sub(t) < time.Minute {
				recent = append(recent, t)
			}
		}
		if len(recent) >= maxRequests {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		userTimestamps[kid.(string)] = append(recent, now)
		c.Next()
	}
}
