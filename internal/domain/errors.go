package domain

import "fmt"

type RuleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details any    `json:"details,omitempty"`
}

func NewDetailedRuleError(code, message, field string, details any) *RuleError {
	return &RuleError{Code: code, Message: message, Field: field, Details: details}
}

func (e *RuleError) Error() string { return e.Message }

func NewRuleError(code, message, field string) *RuleError {
	return &RuleError{Code: code, Message: message, Field: field}
}

func Required(field, label string) error {
	return NewRuleError("required", fmt.Sprintf("%s不能为空", label), field)
}

func Invalid(field, message string) error {
	return NewRuleError("invalid_value", message, field)
}

type ConflictError struct {
	Expected int64
	Actual   int64
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("版本冲突：期望 %d，当前 %d", e.Expected, e.Actual)
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %s 不存在", e.Resource, e.ID)
}
