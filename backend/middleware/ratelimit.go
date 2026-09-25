package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"backend/utils"
)

type clientRecord struct {
	timestamps []time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	records  map[string]*clientRecord
	limit    int
	window   time.Duration
	errorMsg string
}

func NewRateLimiter(limit int, window time.Duration, errorMsg string) *RateLimiter {
	rl := &RateLimiter{
		records:  make(map[string]*clientRecord),
		limit:    limit,
		window:   window,
		errorMsg: errorMsg,
	}

	// Periodic cleanup of stale records every 10 minutes
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for ip, record := range rl.records {
				var valid []time.Time
				for _, t := range record.timestamps {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.records, ip)
				} else {
					record.timestamps = valid
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}
	parts := strings.Split(r.RemoteAddr, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return r.RemoteAddr
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		now := time.Now()
		cutoff := now.Add(-rl.window)

		rl.mu.Lock()
		record, exists := rl.records[ip]
		if !exists {
			record = &clientRecord{}
			rl.records[ip] = record
		}

		// Filter timestamps within window
		var valid []time.Time
		for _, t := range record.timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		record.timestamps = valid

		if len(record.timestamps) >= rl.limit {
			rl.mu.Unlock()
			utils.WriteError(w, http.StatusTooManyRequests, rl.errorMsg)
			return
		}

		record.timestamps = append(record.timestamps, now)
		rl.mu.Unlock()

		next(w, r)
	}
}

// APIRateLimit: 100 requests per 15 minutes
var APIRateLimit = NewRateLimiter(100, 15*time.Minute, "Too many requests from this IP, please try again later.")

// StrictRateLimit: 5 requests per 15 minutes (for login and register)
var StrictRateLimit = NewRateLimiter(5, 15*time.Minute, "Too many attempts from this IP, please try again after 15 minutes.")
