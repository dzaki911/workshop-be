package http

import (
	"context"
	"log"
	"net/http"
	"time"

	"workshop-be/domain"
	"workshop-be/pkg/response"
)

type ctxKey string

const adminIDKey ctxKey = "admin_id"

func adminID(ctx context.Context) int64 {
	id, _ := ctx.Value(adminIDKey).(int64)
	return id
}

// DevAdmin adalah pengganti sementara auth (sesi 2): hanya aktif saat APP_ENV=development
// dan memakai akun admin uji dari seed. Di environment lain semua rute admin ditolak 401
// sampai auth/middleware sesi 3 menggantikannya.
func DevAdmin(enabled bool, adminEmail string, users domain.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				response.Error(w, http.StatusUnauthorized, "Autentikasi belum tersedia", nil)
				return
			}
			u, err := users.GetByEmail(r.Context(), adminEmail)
			if err != nil || u.Role != domain.RoleAdmin {
				response.Error(w, http.StatusUnauthorized, "Admin uji tidak ditemukan; jalankan seed (go run ./cmd/seed)", nil)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminIDKey, u.ID)))
		})
	}
}

// Recover mengubah panic menjadi 500 berformat JSON.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				response.Error(w, http.StatusInternalServerError, "Internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger mencatat method, path, status, dan durasi.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}
