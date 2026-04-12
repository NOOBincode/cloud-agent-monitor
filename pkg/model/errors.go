package model

import (
	"errors"
	"fmt"
	"net/http"
)

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *APIError) WithDetails(details string) *APIError {
	return &APIError{
		Code:      e.Code,
		Message:   e.Message,
		Details:   details,
		RequestID: e.RequestID,
	}
}

func (e *APIError) WithRequestID(requestID string) *APIError {
	return &APIError{
		Code:      e.Code,
		Message:   e.Message,
		Details:   e.Details,
		RequestID: requestID,
	}
}

func (e *APIError) HTTPStatus() int {
	return CodeToHTTPStatus(e.Code)
}

type APIResponse struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

type PagedData struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func SuccessResponse(data interface{}) *APIResponse {
	return &APIResponse{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	}
}

func SuccessWithMessage(message string, data interface{}) *APIResponse {
	return &APIResponse{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	}
}

func PagedResponse(items interface{}, total int64, page, pageSize int) *APIResponse {
	return &APIResponse{
		Code:    CodeSuccess,
		Message: "success",
		Data: &PagedData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}
}

func ErrorResponse(code, message string) *APIResponse {
	return &APIResponse{
		Code:    code,
		Message: message,
	}
}

func ErrorResponseFromError(err error) *APIResponse {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return &APIResponse{
			Code:      apiErr.Code,
			Message:   apiErr.Message,
			Data:      apiErr.Details,
			RequestID: apiErr.RequestID,
		}
	}
	return &APIResponse{
		Code:    CodeInternalError,
		Message: err.Error(),
	}
}

const (
	CodeSuccess             = "SUCCESS"
	CodeBadRequest          = "BAD_REQUEST"
	CodeInvalidRequest      = "INVALID_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeUnprocessableEntity = "UNPROCESSABLE_ENTITY"
	CodeTooManyRequests     = "TOO_MANY_REQUESTS"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
	CodeTokenExpired        = "TOKEN_EXPIRED"
	CodeInvalidToken        = "INVALID_TOKEN"
)

const (
	CodeUserNotFound       = "USER_NOT_FOUND"
	CodeUserExists         = "USER_EXISTS"
	CodeUserInactive       = "USER_INACTIVE"
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeInvalidResetToken  = "INVALID_RESET_TOKEN"
	CodeTokenExpiredReset  = "TOKEN_EXPIRED_RESET"
	CodeInvalidID          = "INVALID_ID"
)

const (
	CodeAlertNotFound   = "ALERT_NOT_FOUND"
	CodeSilenceNotFound = "SILENCE_NOT_FOUND"
	CodeInvalidAlert    = "INVALID_ALERT"
	CodeInvalidSilence  = "INVALID_SILENCE"
)

const (
	CodeAPIKeyNotFound = "API_KEY_NOT_FOUND"
	CodeAPIKeyExpired  = "API_KEY_EXPIRED"
	CodeAPIKeyRevoked  = "API_KEY_REVOKED"
)

func CodeToHTTPStatus(code string) int {
	switch code {
	case CodeSuccess:
		return http.StatusOK
	case CodeBadRequest, CodeInvalidRequest, CodeInvalidID:
		return http.StatusBadRequest
	case CodeUnauthorized, CodeInvalidToken, CodeTokenExpired:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound, CodeUserNotFound, CodeAlertNotFound, CodeSilenceNotFound, CodeAPIKeyNotFound:
		return http.StatusNotFound
	case CodeConflict, CodeUserExists:
		return http.StatusConflict
	case CodeUnprocessableEntity:
		return http.StatusUnprocessableEntity
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	case CodeInternalError:
		return http.StatusInternalServerError
	case CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func NewBadRequestError(message string) *APIError {
	return &APIError{Code: CodeBadRequest, Message: message}
}

func NewInvalidRequestError(message string) *APIError {
	return &APIError{Code: CodeInvalidRequest, Message: message}
}

func NewUnauthorizedError(message string) *APIError {
	return &APIError{Code: CodeUnauthorized, Message: message}
}

func NewForbiddenError(message string) *APIError {
	return &APIError{Code: CodeForbidden, Message: message}
}

func NewNotFoundError(code, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

func NewConflictError(message string) *APIError {
	return &APIError{Code: CodeConflict, Message: message}
}

func NewInternalError(message string) *APIError {
	return &APIError{Code: CodeInternalError, Message: message}
}

func NewServiceUnavailableError(message string) *APIError {
	return &APIError{Code: CodeServiceUnavailable, Message: message}
}

func IsAPIError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr)
}

func GetAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
