package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"admin/server/internal/shared/apperror"
	"github.com/gin-gonic/gin"
	playground "github.com/go-playground/validator/v10"
)

var bindingValidator = newBindingValidator()

func BindJSON(context *gin.Context, target any) error {
	payload, err := io.ReadAll(context.Request.Body)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return apperror.InvalidRequest(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
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

func rejectDuplicateJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("JSON contains trailing data")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is invalid")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("JSON object contains duplicate key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	closingDelimiter, ok := closing.(json.Delim)
	if !ok || (delimiter == '{' && closingDelimiter != '}') || (delimiter == '[' && closingDelimiter != ']') {
		return fmt.Errorf("JSON container is malformed")
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
