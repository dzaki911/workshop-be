package http

import (
	"net/http"

	"workshop-be/domain"
	"workshop-be/pkg/response"
)

type Handler struct {
	users   domain.UserUsecase
	aspects domain.AspectUsecase
	reports domain.ReportUsecase
}

func NewHandler(u domain.UserUsecase, a domain.AspectUsecase, r domain.ReportUsecase) *Handler {
	return &Handler{users: u, aspects: a, reports: r}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	response.Success(w, http.StatusOK, "Server is running", map[string]string{"status": "ok"})
}

// ---- users ----

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListFunctionaries(r.Context(), r.URL.Query().Get("role"))
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Data successfully retrieved", users)
}

// ---- aspects ----

func (h *Handler) ListAspects(w http.ResponseWriter, r *http.Request) {
	list, err := h.aspects.List(r.Context(), r.URL.Query().Get("role_type"))
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Data successfully retrieved", list)
}

func (h *Handler) GetAspect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	a, err := h.aspects.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Data successfully retrieved", a)
}

func (h *Handler) CreateAspect(w http.ResponseWriter, r *http.Request) {
	var in domain.AspectInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.aspects.Create(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Aspect successfully created", a)
}

func (h *Handler) UpdateAspect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in domain.AspectInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.aspects.Update(r.Context(), id, in)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Aspect successfully updated", a)
}

func (h *Handler) DeleteAspect(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.aspects.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Aspect successfully deleted", nil)
}

// ---- reports ----

func (h *Handler) ListReports(w http.ResponseWriter, r *http.Request) {
	list, err := h.reports.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Data successfully retrieved", list)
}

func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rep, err := h.reports.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Data successfully retrieved", rep)
}

func (h *Handler) CreateReport(w http.ResponseWriter, r *http.Request) {
	var in domain.ReportInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	// created_by selalu dari identitas admin sesi, bukan dari body.
	rep, err := h.reports.Create(r.Context(), adminID(r.Context()), in)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Report successfully created", rep)
}

func (h *Handler) UpdateReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in domain.ReportInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	rep, err := h.reports.Update(r.Context(), id, in)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Report successfully updated", rep)
}

// DeleteReport menghapus raport + nilainya; butuh konfirmasi eksplisit ?confirm=true.
func (h *Handler) DeleteReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("confirm") != "true" {
		response.Error(w, http.StatusBadRequest, "Validation failed",
			[]response.FieldError{{Field: "confirm", Message: "Penghapusan raport harus dikonfirmasi dengan ?confirm=true"}})
		return
	}
	if err := h.reports.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Report successfully deleted", nil)
}
