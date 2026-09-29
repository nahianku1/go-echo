package middlewares

import (
	"net-http/internal/utils"
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
)

const ValidatedBodyKey = "validated_body"

// emailRFC5322Re requires a TLD of at least 2 characters, which net/mail.ParseAddress
// does NOT enforce (it accepts single-char TLDs like "foo@bar.c").
var emailRFC5322Re = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[a-zA-Z]{2,}$`)

var validate = func() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("email_rfc5322", func(fl validator.FieldLevel) bool {
		return emailRFC5322Re.MatchString(fl.Field().String())
	})
	return v
}()

// ValidateBody validates incoming JSON body against generic struct type T and
// stores it on the echo context under ValidatedBodyKey.
func ValidateBody[T any]() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			var body T
			if err := echo.BindBody(c, &body); err != nil {
				return utils.Err(400, "Invalid JSON payload")
			}

			if err := validate.Struct(body); err != nil {
				return utils.Err(400, err.Error())
			}

			c.Set(ValidatedBodyKey, body)

			return next(c)
		}
	}
}
