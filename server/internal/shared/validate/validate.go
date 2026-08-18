package validate

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
	playground "github.com/go-playground/validator/v10"
)

var bindingValidator = newBindingValidator()

func BindJSON(context *gin.Context, target any) error {
	decoder := json.NewDecoder(context.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return apperror.InvalidRequest(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return apperror.InvalidRequest(err)
	}
	if err := bindingValidator.Struct(target); err != nil {
		return apperror.InvalidRequest(err)
	}
	return nil
}

func RequireEmptyBody(context *gin.Context) error {
	request := context.Request
	if request.Body == nil {
		return nil
	}
	if request.Body == http.NoBody && request.ContentLength == 0 && len(request.TransferEncoding) == 0 {
		return nil
	}
	if request.ContentLength > 0 {
		return apperror.InvalidRequest(errors.New("request body must be empty"))
	}

	var byteBuffer [1]byte
	readCount, err := request.Body.Read(byteBuffer[:])
	if readCount > 0 {
		return apperror.InvalidRequest(errors.New("request body must be empty"))
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return apperror.InvalidRequest(err)
	}
	return nil
}

func newBindingValidator() *playground.Validate {
	validate := playground.New()
	validate.SetTagName("binding")
	return validate
}
