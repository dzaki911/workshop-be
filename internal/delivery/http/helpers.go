package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"workshop-be/domain"
	"workshop-be/pkg/response"
)

// writeError memetakan error domain ke status HTTP + format error seragam.
func writeError(w http.ResponseWriter, err error) {
	var verrs domain.ValidationErrors
	switch {
	case errors.As(err, &verrs):
		fe := make([]response.FieldError, len(verrs))
		for i, e := range verrs {
			fe[i] = response.FieldError{Field: e.Field, Message: e.Message}
		}
		response.Error(w, http.StatusBadRequest, "Validation failed", fe)
	case errors.Is(err, domain.ErrNotFound):
		response.Error(w, http.StatusNotFound, "Resource not found", nil)
	case errors.Is(err, domain.ErrReportExists),
		errors.Is(err, domain.ErrAspectInUse),
		errors.Is(err, domain.ErrAspectDuplicate):
		response.Error(w, http.StatusConflict, err.Error(), nil)
	default:
		log.Printf("internal error: %v", err)
		response.Error(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}

// decodeJSON membaca body JSON; mengembalikan error validasi bila body rusak.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return domain.ValidationErrors{{Field: "body", Message: "Request body wajib diisi"}}
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return domain.ValidationErrors{{Field: typeErr.Field, Message: "Tipe data tidak valid"}}
		}
		return domain.ValidationErrors{{Field: "body", Message: "Body harus berupa JSON yang valid"}}
	}
	return nil
}

// pathID membaca {id} dari URL; menulis 400 bila bukan bilangan positif.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "Validation failed",
			[]response.FieldError{{Field: "id", Message: "ID harus bilangan bulat positif"}})
		return 0, false
	}
	return id, true
}
