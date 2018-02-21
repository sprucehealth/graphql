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
	err := errors.New(message)
	return FormatError(err)
}

func FormatError(err error) FormattedError {
	switch err := err.(type) {
	case runtime.Error:
		return FormattedError{
			Message:       err.Error(),
			Type:          ErrorTypeInternal,
			StackTrace:    stackTrace(),
			OriginalError: err,
		}
	case FormattedError:
		return err
	case *FormattedError:
		return *err
	case *Error:
		return FormattedError{
			Type:          err.Type,
			Message:       err.Error(),
			Locations:     err.Locations,
			OriginalError: err.OriginalError,
		}
	case Error:
		return FormattedError{
			Type:          err.Type,
			Message:       err.Error(),
			Locations:     err.Locations,
			OriginalError: err.OriginalError,
		}
	default:
		return FormattedError{
			Type:          ErrorTypeInternal,
			Message:       err.Error(),
			Locations:     []location.SourceLocation{},
			OriginalError: err,
		}
	}
}

func FormatPanic(r interface{}) FormattedError {
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
