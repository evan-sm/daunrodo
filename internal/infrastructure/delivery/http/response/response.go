// Package response provides methods for constructing and sending HTTP responses.
package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

// Response represents a standard HTTP response structure.
type Response struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Data    any    `json:"data"`
}

// WriteJSON writes a JSON response with the given status, message, data, and error.
func WriteJSON(w http.ResponseWriter, status int, message string, data any, err error) {
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	resp := Response{
		Message: message,
		Data:    data,
		Error:   errorMsg,
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(bytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

// MarshalJSON used to convert nil into [] or {} in JSON responses.
func (r Response) MarshalJSON() ([]byte, error) {
	type alias Response

	out := alias(r)

	if out.Data == nil {
		out.Data = ""
	}

	if out.Data != nil {
		val := reflect.ValueOf(out.Data)
		switch val.Kind() {
		case reflect.Slice:
			if val.IsNil() {
				out.Data = reflect.MakeSlice(val.Type(), 0, 0).Interface()
			}
		case reflect.Map:
			if val.IsNil() {
				out.Data = reflect.MakeMapWithSize(val.Type(), 0).Interface()
			}
		}
	}

	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return b, nil
}

// OK sends a 200 OK response with the provided message, result, and error.
func OK(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusOK, message, res, err)
}

// NoContent sends a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

// Created sends a 201 Created response with the provided message, result, and error.
func Created(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusCreated, message, res, err)
}

// Accepted sends a 202 Accepted response with the provided message, result, and error.
func Accepted(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusAccepted, message, res, err)
}

// BadRequest sends a 400 Bad Request response with the provided message and error.
func BadRequest(w http.ResponseWriter, message string, err error) {
	WriteJSON(w, http.StatusBadRequest, message, nil, err)
}

// NotFound sends a 404 Not Found response with the provided message and error.
func NotFound(w http.ResponseWriter, message string, err error) {
	WriteJSON(w, http.StatusNotFound, message, nil, err)
}

// Conflict sends a 409 Conflict response with the provided message and error.
func Conflict(w http.ResponseWriter, message string, err error) {
	WriteJSON(w, http.StatusConflict, message, nil, err)
}

// UnprocessableEntity sends a 422 Unprocessable Entity response with the provided message and error.
func UnprocessableEntity(w http.ResponseWriter, message string, err error) {
	WriteJSON(w, http.StatusUnprocessableEntity, message, nil, err)
}

// InternalServerError sends a 500 Internal Server Error response with the provided message, result, and error.
func InternalServerError(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusInternalServerError, message, res, err)
}
