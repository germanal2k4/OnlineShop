package middleware

import (
	"gitlab.ozon.dev/qwestard/homework/internal/audit"
	"log"
	"net/http"
	"time"
)

func BasicAuthMiddleware(user, pass string, methods ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !methodInList(r.Method, methods) {
				next.ServeHTTP(w, r)
				return
			}
			u, p, ok := r.BasicAuth()
			if !ok || u != user || p != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="orders"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func LogMiddleware(auditPool *audit.AuditWorkerPool, methods ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if methodInList(r.Method, methods) {
				log.Printf("[%s] %s", r.Method, r.URL.Path)
				auditPool.Log(audit.AuditLog{
					Timestamp: time.Now().UTC(),
					Endpoint:  r.URL.Path,
					Request:   r.Method + " " + r.URL.String(),
					Message:   "Request received",
				})
			}
			next.ServeHTTP(w, r)
		})
	}
}

func methodInList(method string, methods []string) bool {
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}
