package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"

	"github.com/abdullah-dev5/bill-management/backend/internal/apigen"
	"github.com/abdullah-dev5/bill-management/backend/internal/domain"
)

// apiError is an error with an HTTP status and a stable machine-readable code.
type apiError struct {
	status         int
	code           string
	message        string
	fields         map[string]string
	existingBillID *uuid.UUID
}

func (e *apiError) Error() string { return e.code + ": " + e.message }

func errValidation(fields domain.FieldErrors) *apiError {
	return &apiError{status: http.StatusBadRequest, code: "validation_failed", message: "Some fields need attention.", fields: fields}
}

func errField(field, msg string) *apiError {
	return errValidation(domain.FieldErrors{field: msg})
}

func errNotFound(what string) *apiError {
	return &apiError{status: http.StatusNotFound, code: "not_found", message: "The " + what + " was not found."}
}

func errConflict(code, msg string) *apiError {
	return &apiError{status: http.StatusConflict, code: code, message: msg}
}

func errUnauthorized(code, msg string) *apiError {
	return &apiError{status: http.StatusUnauthorized, code: code, message: msg}
}

func errForbidden(msg string) *apiError {
	return &apiError{status: http.StatusForbidden, code: "forbidden", message: msg}
}

var errTooManyRequests = &apiError{status: http.StatusTooManyRequests, code: "rate_limited", message: "Too many attempts. Please wait a few minutes and try again."}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, e *apiError) {
	body := apigen.Error{Code: e.code, Message: e.message, ExistingBillId: e.existingBillID}
	if len(e.fields) > 0 {
		f := e.fields
		body.Fields = &f
	}
	writeJSON(w, e.status, body)
}

// fail writes err to the client. Unknown errors are logged and hidden behind a 500.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeError(w, ae)
		return
	}
	s.log.ErrorContext(r.Context(), "request failed", slog.String("path", r.URL.Path), slog.Any("err", err))
	writeError(w, &apiError{status: http.StatusInternalServerError, code: "internal", message: "Something went wrong on our side. Please try again."})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return &apiError{status: http.StatusBadRequest, code: "invalid_json", message: "The request body is not valid JSON."}
	}
	return nil
}

// specValidationError turns OpenAPI request-validation errors into field errors.
func specValidationError(err error) *apiError {
	fields := domain.FieldErrors{}
	var walk func(error)
	walk = func(e error) {
		var multi openapi3.MultiError
		if errors.As(e, &multi) {
			for _, inner := range multi {
				walk(inner)
			}
			return
		}
		var reqErr *openapi3filter.RequestError
		if errors.As(e, &reqErr) {
			if reqErr.Parameter != nil {
				fields.Add(reqErr.Parameter.Name, firstLine(reqErr.Reason, reqErr.Err))
				return
			}
			if reqErr.Err != nil {
				walk(reqErr.Err)
				return
			}
		}
		var schemaErr *openapi3.SchemaError
		if errors.As(e, &schemaErr) {
			name := strings.Join(schemaErr.JSONPointer(), ".")
			if name == "" {
				name = "body"
			}
			fields.Add(name, schemaErr.Reason)
			return
		}
		fields.Add("request", e.Error())
	}
	walk(err)
	return errValidation(fields)
}

func firstLine(reason string, err error) string {
	if reason != "" {
		return reason
	}
	if err != nil {
		return strings.SplitN(err.Error(), "\n", 2)[0]
	}
	return "invalid value"
}
