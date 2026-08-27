package control

import (
	"encoding/json"
	"net/http"
)

// Error codes from CONTROL_API.md §2 (closed set).
const (
	CodeUnauthorized       = "unauthorized"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeProfileInvalid     = "profile_invalid"
	CodeGatewayUnavailable = "gateway_unavailable"
	CodeRateLimited        = "rate_limited"
	CodeBadRequest         = "bad_request"
	CodeInternal           = "internal"
)

type errorBody struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	body.Error.Details = details
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
