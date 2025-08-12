package router

import (
	"encoding/json"
	"net/http"
)

const contentTypeJSON = "application/json; charset=utf-8"

// DataEnvelope is the standard success response shape.
type DataEnvelope[T any] struct {
	Data T `json:"data"`
}

// ErrorEnvelope is the standard error response shape.
type ErrorEnvelope struct {
	Error     string `json:"error"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Details   any    `json:"details,omitempty"`
}

// RenderData writes a JSON success response with the given status and data under {"data": ...}.
func RenderData(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	// Avoid generics on the call-site by wrapping here
	_ = json.NewEncoder(w).Encode(DataEnvelope[any]{Data: v})
}

// RenderOK writes a 200 JSON success response.
func RenderOK(w http.ResponseWriter, r *http.Request, v any) {
	RenderData(w, r, http.StatusOK, v)
}

// RenderCreated writes a 201 JSON success response.
func RenderCreated(w http.ResponseWriter, r *http.Request, v any) {
	RenderData(w, r, http.StatusCreated, v)
}

// RenderNoContent writes a 204 response with no body.
func RenderNoContent(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// RenderError writes a JSON error response with a standard shape.
// code is a machine-readable error identifier; message is a human-friendly description.
// details can be any additional payload (validation errors, fields, etc.).
func RenderError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	rid := GetReqID(r.Context())
	if rid == "" {
		rid = r.Header.Get("X-Request-ID")
	}
	env := ErrorEnvelope{Error: code, Message: message, RequestID: rid, Details: details}
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

// Convenience error helpers
func BadRequest(w http.ResponseWriter, r *http.Request, code, message string, details any) {
	RenderError(w, r, http.StatusBadRequest, code, message, details)
}

func Unauthorized(w http.ResponseWriter, r *http.Request, code, message string) {
	RenderError(w, r, http.StatusUnauthorized, code, message, nil)
}

func Forbidden(w http.ResponseWriter, r *http.Request, code, message string) {
	RenderError(w, r, http.StatusForbidden, code, message, nil)
}

func NotFound(w http.ResponseWriter, r *http.Request, code, message string) {
	RenderError(w, r, http.StatusNotFound, code, message, nil)
}

func Conflict(w http.ResponseWriter, r *http.Request, code, message string) {
	RenderError(w, r, http.StatusConflict, code, message, nil)
}

func UnprocessableEntity(w http.ResponseWriter, r *http.Request, code, message string, details any) {
	RenderError(w, r, http.StatusUnprocessableEntity, code, message, details)
}

func InternalError(w http.ResponseWriter, r *http.Request, code, message string) {
	RenderError(w, r, http.StatusInternalServerError, code, message, nil)
}
