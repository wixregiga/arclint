// Package httpkit carries the JSON plumbing the app layer speaks:
// one response envelope, one error shape, one decoder.
package httpkit

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody is the one error shape the box office speaks.
type ErrorBody struct {
	Error string `json:"error"`
}

// Respond writes v as JSON with the given status.
func Respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("httpkit: encoding response", "error", err)
	}
}

// Error writes the error shape with the given status.
func Error(w http.ResponseWriter, status int, msg string) {
	Respond(w, status, ErrorBody{Error: msg})
}

// Decode reads the request body as strict JSON into dst.
func Decode[T any](r *http.Request, dst *T) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
