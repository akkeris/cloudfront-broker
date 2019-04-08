package broker

import (
	"errors"
	"net/http"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func ConflictErrorWithMessage(description string) error {
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusConflict,
		Description: &description,
	}
}

func UnprocessableEntityWithMessage(errMsg string, description string) error {
	return osb.HTTPStatusCodeError{
		ResponseError: errors.New(errMsg),
		StatusCode:    http.StatusUnprocessableEntity,
		Description:   &description,
	}
}

func UnprocessableEntity() error {
	description := "Un-processable Entity"
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusUnprocessableEntity,
		Description: &description,
	}
}

func BadRequestError(d string) error {
	errorMessage := "StatusBadRequest"
	return osb.HTTPStatusCodeError{
		StatusCode:   http.StatusBadRequest,
		ErrorMessage: &errorMessage,
		Description:  &d,
	}
}

func NotFound() error {
	description := "Not Found"
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusNotFound,
		Description: &description,
	}
}

func NotFoundWithMessage(errMsg string, description string) error {
	return osb.HTTPStatusCodeError{
		ResponseError: errors.New(errMsg),
		StatusCode:    http.StatusNotFound,
		Description:   &description,
	}

}
func InternalServerErr() error {
	description := "Internal Server Error"
	return osb.HTTPStatusCodeError{
		StatusCode:  http.StatusInternalServerError,
		Description: &description,
	}
}

func InternalServerErrWithMessage(errMsg string, description string) error {
	return osb.HTTPStatusCodeError{
		ResponseError: errors.New(errMsg),
		StatusCode:    http.StatusInternalServerError,
		Description:   &description,
	}
}
