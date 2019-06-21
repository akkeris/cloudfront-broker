package broker

import (
	"errors"
	"net/http"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

// ConflictErrorWithMessage return OSB conflict error
func ConflictErrorWithMessage(description string) error {
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusConflict,
		Description: &description,
	}
}

// UnprocessableEntityWithMessage returns OSB unprocessable error with passwd message
func UnprocessableEntityWithMessage(errMsg string, description string) error {
	return osb.HTTPStatusCodeError{
		ResponseError: errors.New(errMsg),
		StatusCode:    http.StatusUnprocessableEntity,
		Description:   &description,
	}
}

// UnprocessableEntity returns OSB unprocessable error
func UnprocessableEntity() error {
	description := "Un-processable Entity"
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusUnprocessableEntity,
		Description: &description,
	}
}

// BadRequestError returns OSB bad request error
func BadRequestError(d string) error {
	errorMessage := "StatusBadRequest"
	return osb.HTTPStatusCodeError{
		StatusCode:   http.StatusBadRequest,
		ErrorMessage: &errorMessage,
		Description:  &d,
	}
}

// NotFound returns OSB not found error
func NotFound() error {
	description := "Not Found"
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusNotFound,
		Description: &description,
	}
}

//NotFoundWithMessage returns OSB not found error with passed message
func NotFoundWithMessage(errMsg string, description string) error {
	return osb.HTTPStatusCodeError{
		ResponseError: errors.New(errMsg),
		StatusCode:    http.StatusNotFound,
		Description:   &description,
	}

}

// InternalServerErr returns OSB internal server error
func InternalServerErr() error {
	description := "Internal Server Error"
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusInternalServerError,
		Description: &description,
	}
}

// InternalServerErrWithMessage returns internal server error with passed message
func InternalServerErrWithMessage(errMsg string, description string) error {
	return osb.HTTPStatusCodeError{
		ResponseError: errors.New(errMsg),
		StatusCode:    http.StatusInternalServerError,
		Description:   &description,
	}
}
