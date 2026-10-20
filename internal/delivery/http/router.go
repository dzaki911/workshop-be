package http

import (
	"net/http"

	"workshop-be/pkg/response"
)

// NewRouter menyusun routing /api/v1. adminOnly membungkus rute admin
// (sesi 2: DevAdmin; sesi 3: middleware JWT + cek role).
func NewRouter(h *Handler, adminOnly func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)

	// Auth: dilengkapi pada sesi 3.
	for _, p := range []string{
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"GET /api/v1/me",
		"GET /api/v1/me/report",
	} {
		mux.HandleFunc(p, notImplemented)
	}

	admin := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, adminOnly(fn))
	}
	admin("GET /api/v1/admin/users", h.ListUsers)

	admin("GET /api/v1/admin/aspects", h.ListAspects)
	admin("POST /api/v1/admin/aspects", h.CreateAspect)
	admin("GET /api/v1/admin/aspects/{id}", h.GetAspect)
	admin("PUT /api/v1/admin/aspects/{id}", h.UpdateAspect)
	admin("DELETE /api/v1/admin/aspects/{id}", h.DeleteAspect)

	admin("GET /api/v1/admin/reports", h.ListReports)
	admin("POST /api/v1/admin/reports", h.CreateReport)
	admin("GET /api/v1/admin/reports/{id}", h.GetReport)
	admin("PUT /api/v1/admin/reports/{id}", h.UpdateReport)
	admin("DELETE /api/v1/admin/reports/{id}", h.DeleteReport)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		response.Error(w, http.StatusNotFound, "Resource not found", nil)
	})
	return Logger(Recover(mux))
}

func notImplemented(w http.ResponseWriter, _ *http.Request) {
	response.Error(w, http.StatusNotImplemented, "Endpoint belum diimplementasikan", nil)
}
