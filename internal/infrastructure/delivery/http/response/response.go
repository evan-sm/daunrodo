package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Data    any    `json:"data"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, message string, data any, err error) {
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	r := Response{
		Message: message,
		Data:    data,
		Error:   errorMsg,
	}

	bytes, err := json.Marshal(r)
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

func OK(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusOK, message, res, err)
}

func NoContent(w http.ResponseWriter) {
	WriteJSON(w, http.StatusNoContent, "no content", nil, nil)
}

func Created(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusCreated, message, res, err)
}

func Accepted(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusAccepted, message, res, err)
}

func BadRequest(w http.ResponseWriter, message string, err error) {
	WriteJSON(w, http.StatusBadRequest, message, nil, err)
}

func UnprocessableEntity(w http.ResponseWriter, message string, err error) {
	WriteJSON(w, http.StatusUnprocessableEntity, message, nil, err)
}

func InternalServerError(w http.ResponseWriter, message string, res any, err error) {
	WriteJSON(w, http.StatusInternalServerError, message, res, err)
}
