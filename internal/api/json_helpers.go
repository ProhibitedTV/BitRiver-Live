package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxJSONBodyBytes = 1 << 20 // 1 MiB

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiErrorResponse struct {
	Error apiErrorBody `json:"error"`
}

type codedError interface {
	Code() string
}

type statusError interface {
	StatusCode() int
}

type clientMessageError interface {
	ClientMessage() string
}

// RequestError captures a structured API error with a status code and machine-readable code.
type RequestError struct {
	Status  int
	CodeVal string
	Message string
	Err     error
}

func (e RequestError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.StatusCode())
}

// Unwrap surfaces the wrapped error for errors.Is/errors.As handling.
func (e RequestError) Unwrap() error {
	return e.Err
}

// Code returns the machine-readable code for the error.
func (e RequestError) Code() string {
	if e.CodeVal != "" {
		return e.CodeVal
	}
	return errorCodeForStatus(e.StatusCode())
}

// StatusCode returns the HTTP status associated with the error.
func (e RequestError) StatusCode() int {
	if e.Status != 0 {
		return e.Status
	}
	return http.StatusInternalServerError
}

func (e RequestError) ClientMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Error()
}

// WriteJSON writes a JSON payload with the provided status code.
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteError writes a structured error payload using the provided status code.
func WriteError(w http.ResponseWriter, status int, err error) {
	code := errorCodeForStatus(status)
	if coder, ok := err.(codedError); ok {
		if c := coder.Code(); c != "" {
			code = c
		}
	}

	message := clientMessage(status, err)
	WriteJSON(w, status, apiErrorResponse{Error: apiErrorBody{Code: code, Message: message}})
}

// WriteDecodeError normalises JSON decoding failures to the correct HTTP status and code.
func WriteDecodeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	if serr, ok := err.(statusError); ok {
		WriteError(w, serr.StatusCode(), err)
		return
	}

	WriteError(w, http.StatusBadRequest, err)
}

// DecodeJSON parses a JSON payload into dest, rejecting unknown fields and enforcing a body size limit.
func DecodeJSON(r *http.Request, dest interface{}) error {
	return decodeJSON(r, dest, true)
}

// DecodeJSONAllowUnknown parses a JSON payload into dest while allowing unknown fields.
func DecodeJSONAllowUnknown(r *http.Request, dest interface{}) error {
	return decodeJSON(r, dest, false)
}

func decodeJSON(r *http.Request, dest interface{}, disallowUnknown bool) error {
	if r.Body == nil {
		return RequestError{Status: http.StatusBadRequest, CodeVal: "validation_failed", Message: "request body is required"}
	}
	defer r.Body.Close()

	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil {
		return RequestError{Status: http.StatusBadRequest, CodeVal: "invalid_json", Message: "unable to read request body", Err: err}
	}

	if len(body) == 0 {
		return RequestError{Status: http.StatusBadRequest, CodeVal: "validation_failed", Message: "request body is required"}
	}

	if len(body) > maxJSONBodyBytes {
		return RequestError{Status: http.StatusRequestEntityTooLarge, CodeVal: "request_too_large", Message: fmt.Sprintf("request body must not exceed %d bytes", maxJSONBodyBytes)}
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if disallowUnknown {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(dest); err != nil {
		return classifyDecodeError(err)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return classifyDecodeError(err)
	}

	return nil
}

// DecodeAndValidate parses a JSON payload into dest and writes a structured error response on failure.
//
// It returns true when decoding succeeds, otherwise it writes a decode error response and returns false.
func DecodeAndValidate(w http.ResponseWriter, r *http.Request, dest interface{}) bool {
	if err := DecodeJSON(r, dest); err != nil {
		WriteDecodeError(w, err)
		return false
	}
	return true
}

// DecodeAllowUnknownAndValidate parses a JSON payload while allowing unknown fields and writes a structured error response on failure.
//
// It returns true when decoding succeeds, otherwise it writes a decode error response and returns false.
func DecodeAllowUnknownAndValidate(w http.ResponseWriter, r *http.Request, dest interface{}) bool {
	if err := DecodeJSONAllowUnknown(r, dest); err != nil {
		WriteDecodeError(w, err)
		return false
	}
	return true
}

func classifyDecodeError(err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	switch {
	case errors.As(err, &syntaxErr):
		return RequestError{Status: http.StatusBadRequest, CodeVal: "invalid_json", Message: "malformed JSON", Err: err}
	case errors.Is(err, io.ErrUnexpectedEOF):
		return RequestError{Status: http.StatusBadRequest, CodeVal: "invalid_json", Message: "malformed JSON", Err: err}
	case errors.As(err, &typeErr):
		if typeErr.Field != "" {
			return RequestError{Status: http.StatusBadRequest, CodeVal: "validation_failed", Message: fmt.Sprintf("invalid value for %s", typeErr.Field), Err: err}
		}
		return RequestError{Status: http.StatusBadRequest, CodeVal: "validation_failed", Message: "invalid value", Err: err}
	case errors.Is(err, io.EOF):
		return RequestError{Status: http.StatusBadRequest, CodeVal: "invalid_json", Message: "request body cannot be empty", Err: err}
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return RequestError{Status: http.StatusBadRequest, CodeVal: "validation_failed", Message: fmt.Sprintf("unknown field %s", field), Err: err}
	default:
		return RequestError{Status: http.StatusBadRequest, CodeVal: "invalid_json", Message: "invalid JSON payload", Err: err}
	}
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusRequestEntityTooLarge:
		return "request_too_large"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "error"
	}
}

func clientMessage(status int, err error) string {
	if msgErr, ok := err.(clientMessageError); ok {
		if msg := msgErr.ClientMessage(); msg != "" {
			return msg
		}
	}

	if status >= http.StatusInternalServerError {
		return http.StatusText(status)
	}

	if err != nil {
		return err.Error()
	}

	return http.StatusText(status)
}

// WriteRequestError writes a structured error response using the status inferred from the error when possible.
func WriteRequestError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if serr, ok := err.(statusError); ok {
		status = serr.StatusCode()
	}
	WriteError(w, status, err)
}

// WriteMethodNotAllowed writes a consistent 405 response and populates the Allow header.
func WriteMethodNotAllowed(w http.ResponseWriter, r *http.Request, allowed ...string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
	}
	WriteRequestError(w, RequestError{
		Status:  http.StatusMethodNotAllowed,
		CodeVal: "method_not_allowed",
		Message: fmt.Sprintf("method %s not allowed", r.Method),
	})
}

// ValidationError builds a RequestError for invalid user input.
func ValidationError(message string) RequestError {
	return RequestError{Status: http.StatusBadRequest, CodeVal: "validation_failed", Message: message}
}

// ServiceUnavailableError builds a RequestError for temporarily unavailable services.
func ServiceUnavailableError(message string) RequestError {
	return RequestError{Status: http.StatusServiceUnavailable, CodeVal: "service_unavailable", Message: message}
}
