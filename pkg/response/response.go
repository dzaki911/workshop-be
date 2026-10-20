package response

import (
	"encoding/json"
	"net/http"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Body adalah format JSON seragam untuk semua endpoint.
type Body struct {
	Status  string       `json:"status"` // "success" | "error"
	Message string       `json:"message"`
	Data    any          `json:"data,omitempty"`
	Errors  []FieldError `json:"errors,omitempty"`
}

func write(w http.ResponseWriter, code int, b Body) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(b)
}

func Success(w http.ResponseWriter, code int, message string, data any) {
	write(w, code, Body{Status: "success", Message: message, Data: data})
}

func Error(w http.ResponseWriter, code int, message string, errs []FieldError) {
	write(w, code, Body{Status: "error", Message: message, Errors: errs})
}
