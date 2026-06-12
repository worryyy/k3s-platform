package bizerr

import "net/http"

type Error struct {
	Code    int
	Message string
	Cause   error
}

func New(code int, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func Param(message string) *Error {
	return New(http.StatusBadRequest, message, nil)
}

func Biz(message string) *Error {
	return New(http.StatusBadRequest, message, nil)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, message, nil)
}

func Internal(message string) *Error {
	return New(http.StatusInternalServerError, message, nil)
}

func ParamWrap(message string, err error) *Error {
	return New(http.StatusBadRequest, message, err)
}

func BizWrap(message string, err error) *Error {
	return New(http.StatusBadRequest, message, err)
}

func NotFoundWrap(message string, err error) *Error {
	return New(http.StatusNotFound, message, err)
}

func InternalWrap(message string, err error) *Error {
	return New(http.StatusInternalServerError, message, err)
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if text := http.StatusText(e.Code); text != "" {
		return text
	}
	return "error"
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
