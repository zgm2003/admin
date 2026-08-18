package middleware

import (
	"fmt"
	"strings"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func Language() gin.HandlerFunc {
	return func(context *gin.Context) {
		locale := i18n.ZhCN
		acceptLanguage := context.GetHeader("Accept-Language")
		if acceptLanguage != "" {
			firstRange := strings.TrimSpace(strings.SplitN(acceptLanguage, ",", 2)[0])
			languageTag := strings.TrimSpace(strings.SplitN(firstRange, ";", 2)[0])
			parsed, err := i18n.ParseLocale(languageTag)
			if err != nil {
				setRequestLocale(context, i18n.ZhCN)
				response.Fail(context, apperror.InvalidRequest(fmt.Errorf("parse Accept-Language: %w", err)))
				return
			}
			locale = parsed
		}

		setRequestLocale(context, locale)
		context.Next()
	}
}

func setRequestLocale(context *gin.Context, locale i18n.Locale) {
	context.Request = context.Request.WithContext(i18n.WithLocale(context.Request.Context(), locale))
	context.Header("Content-Language", string(locale))
}
