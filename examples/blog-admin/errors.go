package main

import "fmt"

// HTTPError represents an HTTP error with a status code
type HTTPError struct {
	Code    int
	Message string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Message)
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(code int, message string) HTTPError {
	return HTTPError{Code: code, Message: message}
}
