package validate

import (
	"encoding/json"
	"io"

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

func newBindingValidator() *playground.Validate {
	validate := playground.New()
	validate.SetTagName("binding")
	return validate
}
