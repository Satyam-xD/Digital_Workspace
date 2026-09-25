package middleware

import (
	"net/http"
	"os"
	"strings"
)

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://localhost:4000",
	"http://127.0.0.1:5173",
}

// IsOriginAllowed checks if the origin matches allowed origins or ends with .vercel.app
func IsOriginAllowed(origin string) bool {
	if origin == "" {
		return true
	}

	clientURL := os.Getenv("CLIENT_URL")
	if clientURL != "" && origin == clientURL {
		return true
	}

	for _, o := range defaultAllowedOrigins {
		if origin == o {
			return true
		}
	}

	if strings.HasSuffix(origin, ".vercel.app") {
		return true
	}

	return false
}

// CORSHandler handles CORS headers and preflight OPTIONS requests
func CORSHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if IsOriginAllowed(origin) {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
