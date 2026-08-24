package domain

import "fmt"

type ErrorCode string

const (
	ErrValidation       ErrorCode = "VALIDATION_FAILED"
	ErrNotFound         ErrorCode = "NOT_FOUND"
	ErrConflict         ErrorCode = "VERSION_CONFLICT"
	ErrInvalidState     ErrorCode = "INVALID_STATE"
	ErrDuplicate        ErrorCode = "DUPLICATE_CONTENT"
	ErrForbidden        ErrorCode = "FORBIDDEN"
	ErrIncomplete       ErrorCode = "INCOMPLETE_BATCH"
	ErrAlreadyPublished ErrorCode = "ALREADY_PUBLISHED"
)

type DomainError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

func (e *DomainError) Error() string { return string(e.Code) + ": " + e.Message }

func NewError(code ErrorCode, message string) error {
	return &DomainError{Code: code, Message: message}
}

func FieldError(field, message string) error {
	return &DomainError{Code: ErrValidation, Message: message, Field: field}
}

func AsDomainError(err error) *DomainError {
	if de, ok := err.(*DomainError); ok {
		return de
	}
	return &DomainError{Code: ErrValidation, Message: fmt.Sprintf("%v", err)}
}
