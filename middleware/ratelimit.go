package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LoginRateLimit 登录接口限流中间件
// 每 IP 每分钟最多 maxAttempts 次尝试，超过则返回 429
func LoginRateLimit(maxAttempts int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		timestamps []time.Time
	}
	var (
		mu    sync.RWMutex
		store = make(map[string]*entry)
	)
	// 定期清理过期数据
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			cutoff := time.Now().Add(-window)
			for ip, e := range store {
				newTs := e.timestamps[:0]
				for _, t := range e.timestamps {
					if t.After(cutoff) {
						newTs = append(newTs, t)
					}
				}
				if len(newTs) == 0 {
					delete(store, ip)
				} else {
					e.timestamps = newTs
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		mu.Lock()
		e, ok := store[ip]
		if !ok {
			e = &entry{}
			store[ip] = e
		}
		// 移除窗口外的记录
		cutoff := now.Add(-window)
		newTs := e.timestamps[:0]
		for _, t := range e.timestamps {
			if t.After(cutoff) {
				newTs = append(newTs, t)
			}
		}
		e.timestamps = newTs
		if len(e.timestamps) >= maxAttempts {
			mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "登录尝试过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}
		e.timestamps = append(e.timestamps, now)
		mu.Unlock()
		c.Next()
	}
}
