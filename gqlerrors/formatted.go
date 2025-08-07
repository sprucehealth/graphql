package gqlerrors

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/sprucehealth/graphql/language/location"
)

type FormattedError struct {
	Message       string                    `json:"message"`
	Type          ErrorType                 `json:"type,omitempty"`
	UserMessage   string                    `json:"userMessage,omitempty"`
	Locations     []location.SourceLocation `json:"locations"`
	StackTrace    string                    `json:"-"`
	OriginalError error                     `json:"-"`
}

func (g FormattedError) Error() string {
	return g.Message
}

func NewFormattedError(message string) FormattedError {
	return FormatError(errors.New(message))
}

func FormatError(err error) FormattedError {
	if e, ok := asType[FormattedError](err); ok {
		return e
	}
	if e, ok := asType[*FormattedError](err); ok {
		return *e
	}
	if e, ok := asType[runtime.Error](err); ok {
		return FormattedError{
			Message:       e.Error(),
			Type:          ErrorTypeInternal,
			StackTrace:    stackTrace(),
			OriginalError: e,
		}
	}
	if e, ok := asType[*Error](err); ok {
		return FormattedError{
			Type:          e.Type,
			Message:       e.Error(),
			Locations:     e.Locations,
			OriginalError: e.OriginalError,
		}
	}
	if e, ok := asType[Error](err); ok {
		return FormattedError{
			Type:          e.Type,
			Message:       e.Error(),
			Locations:     e.Locations,
			OriginalError: e.OriginalError,
		}
	}
	return FormattedError{
		Type:          ErrorTypeInternal,
		Message:       err.Error(),
		Locations:     []location.SourceLocation{},
		StackTrace:    stackTrace(),
		OriginalError: err,
	}
}

func FormatPanic(r any) FormattedError {
	if e, ok := r.(FormattedError); ok {
		return e
	}
	return FormattedError{
		Message:    fmt.Sprintf("panic %v", r),
		Type:       ErrorTypeInternal,
		StackTrace: stackTrace(),
	}
}

func FormatErrors(errs ...error) []FormattedError {
	formattedErrors := []FormattedError{}
	for _, err := range errs {
		formattedErrors = append(formattedErrors, FormatError(err))
	}
	return formattedErrors
}

func stackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
