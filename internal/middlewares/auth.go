package middlewares

import (
	"net/http"
	"slices"
	"strings"

	"net-http/internal/utils"

	"github.com/labstack/echo/v5"
)

const UserContextKey = "user"

// Auth guards handlers that require a signed-in user.
//
// The token verifier is injected instead of being called through the old
// package-level utils.VerifyToken helper. That decouples the guard from where the
// signing secret lives (global config, env, key file) and from the concrete JWT
// library: a test can pass a fake verifier that accepts a hard-coded token, and
// the production signer is chosen once, in main.
type Auth struct {
	tokens utils.TokenService
}

// NewAuth builds the guard around an injected token service.
func NewAuth(tokens utils.TokenService) *Auth {
	return &Auth{tokens: tokens}
}

// Require returns an echo middleware that extracts the JWT token from the
// Authorization header or the access_token cookie, validates it as standard or
// OAuth claims, optionally enforces the required roles, and attaches the claims
// to the echo context.
func (a *Auth) Require(requiredRoles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			var token string

			// 1. Extract Bearer token from Authorization header
			// utils.DebugLogger.Printf("From Auth:-[authHeader]:%s", c.Request().Header.Get("Authorization"))
			if authHeader := c.Request().Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}

			utils.DebugLogger.Printf("Token from Mobile:%s", token)

			// 2. Fallback to token cookie
			if token == "" {
				if cookie, err := c.Cookie("access_token"); err == nil {
					token = cookie.Value
				}
			}

			if token == "" {
				return utils.Err(http.StatusUnauthorized, "Authentication token missing")
			}

			// 3. Validate token as standard Claims
			claims, err := a.tokens.VerifyAccessToken(token)
			if err == nil && claims.UserID != 0 {
				if len(requiredRoles) > 0 && !slices.Contains(requiredRoles, claims.UserRole) {
					return utils.Err(http.StatusForbidden, "You are not Authorized!")
				}

				// Store validated user claims on the echo context
				c.Set(UserContextKey, claims)
				return next(c)
			}

			// 4. Fallback: Validate token as OauthClaims
			oauthClaims, oauthErr := a.tokens.VerifyOAuthToken(token)
			if oauthErr == nil && oauthClaims.Provider_id != "" {
				if len(requiredRoles) > 0 {
					return utils.Err(http.StatusForbidden, "You are not Authorized!")
				}

				// Store validated OAuth claims on the echo context
				c.Set(UserContextKey, oauthClaims)
				return next(c)
			}

			return utils.Err(http.StatusUnauthorized, "Invalid or expired token")
		}
	}
}
