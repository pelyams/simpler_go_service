package domain

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("product not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInternalDb    = errors.New("internal database error")
	ErrInternalCache = errors.New("internal cache error")
)

type ErrorContainer struct {
	inner []error
}

func NewErrorContainer(e ...error) ErrorContainer {
	ec := ErrorContainer{inner: make([]error, 0)}
	for _, e := range e {
		ec.inner = append(ec.inner, e)
	}
	return ec
}

func (c *ErrorContainer) Add(e ...error) {
	for _, e := range e {
		c.inner = append(c.inner, e)
	}
}

func (c ErrorContainer) Error() string {
	errMessage := ""
	if c.inner != nil {
		for _, err := range c.inner {
			errMessage = fmt.Sprintf("%s%s;\n", errMessage, err.Error())
		}
	}
	return errMessage
}

func (c ErrorContainer) Unwrap() []error {
	return c.inner
}

type ServiceError struct {
	CriticalError     error
	NonCriticalErrors []error
}

func NewServiceError(critical error, nonCritical []error) *ServiceError {
	return &ServiceError{CriticalError: critical, NonCriticalErrors: nonCritical}
}

func (se *ServiceError) Error() string {
	var errMessage bytes.Buffer
	errMessage.WriteString("Service error(s)\n:")
	for _, err := range se.NonCriticalErrors {
		errMessage.WriteString(fmt.Sprintf("%s\n", err.Error()))
	}
	if se.CriticalError != nil {
		errMessage.WriteString(fmt.Sprintf("%s\n", se.CriticalError.Error()))
	}
	return errMessage.String()
}
