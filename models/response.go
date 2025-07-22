package models

type HttpResponse struct {
	Message string      `json:"message,omitempty"`
	Body    interface{} `json:"body,omitempty"`
	Code    uint32      `json:"code,omitempty"`
	Token   string      `json:"token,omitempty"`
	Error   string      `json:"error,omitempty"`
}
