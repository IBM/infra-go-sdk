package svc

// ErrorResponse represents a possible error response from the API
type ErrorResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}
