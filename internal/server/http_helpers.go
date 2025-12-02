package server

import (
	"net/http"

	"bitriver-live/internal/api"
)

// writeMiddlewareError normalises middleware error responses to the API JSON shape.
func writeMiddlewareError(w http.ResponseWriter, status int, message string) {
	api.WriteError(w, status, api.RequestError{Status: status, Message: message})
}
