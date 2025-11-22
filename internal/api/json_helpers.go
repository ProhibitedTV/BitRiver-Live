package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// WriteError is an exported helper for returning JSON API errors.
func WriteError(w http.ResponseWriter, status int, err error) {
	writeError(w, status, err)
}

func decodeJSON(r *http.Request, dest interface{}) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return nil
}

func decodeJSONAllowUnknown(r *http.Request, dest interface{}) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return nil
}
