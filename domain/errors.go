package domain

import (
	"errors"
	"strings"
)

var (
	ErrNotFound        = errors.New("resource not found")
	ErrReportExists    = errors.New("fungsionaris ini sudah memiliki raport")
	ErrAspectInUse     = errors.New("aspek sudah digunakan dalam raport")
	ErrAspectDuplicate = errors.New("aspek dengan nama dan role_type yang sama sudah ada")
)

// FieldError menjelaskan satu kesalahan validasi pada satu field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors adalah kumpulan FieldError; dipetakan ke HTTP 400.
type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	parts := make([]string, len(v))
	for i, e := range v {
		parts[i] = e.Field + ": " + e.Message
	}
	return strings.Join(parts, "; ")
}

func (v *ValidationErrors) Add(field, message string) {
	*v = append(*v, FieldError{Field: field, Message: message})
}
