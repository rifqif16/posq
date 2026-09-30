// Package httpx berisi helper HTTP bersama: error problem+json (RFC 9457), JSON, middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

const maxBodyBytes = 1 << 20

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Problem struct {
	Type    string       `json:"type"`
	Title   string       `json:"title"`
	Status  int          `json:"status"`
	Detail  string       `json:"detail,omitempty"`
	Code    string       `json:"code"`
	Errors  []FieldError `json:"errors,omitempty"`
	TraceID string       `json:"trace_id,omitempty"`
}

// WriteProblem menulis error seragam. detail tidak boleh berisi error internal.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, fields ...FieldError) {
	p := Problem{
		Type:    "about:blank",
		Title:   http.StatusText(status),
		Status:  status,
		Detail:  detail,
		Code:    code,
		Errors:  fields,
		TraceID: middleware.GetReqID(r.Context()),
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

var ErrBadBody = errors.New("body JSON tidak valid")

// DecodeJSON membatasi ukuran body dan menolak field tak dikenal.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrBadBody
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ErrBadBody
	}
	return nil
}
