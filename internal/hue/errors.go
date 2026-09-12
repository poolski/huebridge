package hue

import (
	"encoding/json"
	"net/http"
)

type SuccessItem struct {
	Success map[string]any `json:"success"`
}

type ErrorBody struct {
	Type        int    `json:"type"`
	Address     string `json:"address"`
	Description string `json:"description"`
}

type ErrorItem struct {
	Error ErrorBody `json:"error"`
}

// WriteError writes the standard CLIP v1 single-error array response.
func WriteError(w http.ResponseWriter, status int, errType int, address, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode([]ErrorItem{{Error: ErrorBody{
		Type:        errType,
		Address:     address,
		Description: description,
	}}})
}

// WriteSuccess writes a single-item CLIP v1 success array response.
func WriteSuccess(w http.ResponseWriter, fields map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]SuccessItem{{Success: fields}})
}
