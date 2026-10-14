package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"net-http/internal/config"
	"net-http/internal/middlewares"
	"net-http/internal/modules/auth/dto"
	"net-http/internal/utils"

	"github.com/labstack/echo/v5"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// Controller is the HTTP layer for the auth module.
//
// It no longer talks to the package-level AuthService singleton, the
// frontendBaseURL constant or the googleConfig global: the use-case service, the
// token signer, the OAuth client and the frontend URL all arrive through
// NewController. Keeping those as fields means this layer only handles HTTP
// concerns and a test can drive it with a stubbed service and OAuth config.
type Controller struct {
	service        *Service
	tokens         utils.TokenService
	oauth          *oauth2.Config
	googleClientID string
	frontendURL    string
}

// NewController injects the controller's collaborators. frontendURL is where
// browser-facing redirects (email verification, OAuth completion) send the user,
// and googleClientID is the audience checked when validating Google ID tokens.
func NewController(service *Service, tokens utils.TokenService, oauth *oauth2.Config, googleClientID, frontendURL string) *Controller {
	return &Controller{
		service:        service,
		tokens:         tokens,
		oauth:          oauth,
		googleClientID: googleClientID,
		frontendURL:    frontendURL,
	}
}

func (c *Controller) Register(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.RegisterRequest](ctx, middlewares.ValidatedBodyKey)
	platform, _ := echo.ContextGet[string](ctx, middlewares.PlatformContextKey)

	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}
	_ = fmt.Sprintf("Payload: %+v\n", payload)

	userResponse, err := c.service.Register(ctx.Request().Context(), payload, platform)
	if err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusCreated, "User registered successfully! Please check your email to activate your account.", userResponse)
}

func (c *Controller) VerifyEmail(ctx *echo.Context) error {
	token := ctx.QueryParam("token")
	platform := ctx.QueryParam("platform")

	if token == "" {
		return utils.Err(http.StatusBadRequest, "token is required")
	}

	var redirectURL *url.URL

	switch platform {
	case "web":
		redirectURL, _ = url.Parse(c.frontendURL + "/verify")

	case "mobile":
		redirectURL, _ = url.Parse(
			config.AppConfig.EXPO_SCHEME + "://verify",
		)

	default:
		return utils.Err(http.StatusBadRequest, "invalid platform")
	}

	q := redirectURL.Query()

	if err := c.service.VerifyEmail(
		ctx.Request().Context(),
		token,
	); err != nil {
		q.Set("status", "error")
		q.Set("message", err.Error())

		redirectURL.RawQuery = q.Encode()

		return ctx.Redirect(
			http.StatusSeeOther,
			redirectURL.String(),
		)
	}

	q.Set("status", "success")
	redirectURL.RawQuery = q.Encode()

	return ctx.Redirect(
		http.StatusSeeOther,
		redirectURL.String(),
	)
}

func (c *Controller) ResendActivationEmail(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.ResendActivationRequest](ctx, middlewares.ValidatedBodyKey)
	platform, _ := echo.ContextGet[string](ctx, middlewares.PlatformContextKey)

	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	if err := c.service.ResendActivationEmail(ctx.Request().Context(), payload, platform); err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[any](ctx, http.StatusOK, "Activation email resent successfully! Please check your inbox.", nil)
}

func (c *Controller) Login(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.LoginRequest](ctx, middlewares.ValidatedBodyKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	err = c.service.Login(ctx.Request().Context(), payload)
	if err != nil {
		return utils.Err(http.StatusUnauthorized, err.Error())
	}

	return utils.SendResponse[*dto.LoginResponse](ctx, http.StatusOK, "OTP sent to your email", nil)
}

func (c *Controller) GoogleLogin(ctx *echo.Context) error {
	// Generate a fresh PKCE verifier for this authorization request.
	verifier := oauth2.GenerateVerifier()

	// In production, store this verifier in a secure,
	// short-lived session/cookie associated with the browser.
	utils.SetCookie(ctx, "oauth_verifier", verifier, 300)

	// State protects against CSRF.
	state := oauth2.GenerateVerifier()

	utils.SetCookie(ctx, "oauth_state", state, 300)

	// Nonce protects against replay attack.
	nonce := oauth2.GenerateVerifier()

	utils.SetCookie(ctx, "oauth_nonce", nonce, 300)

	authURL := c.oauth.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("nonce", nonce),
	)

	return ctx.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (c *Controller) GoogleCallback(ctx *echo.Context) error {

	ctxr := ctx.Request().Context()

	// ------------------------------------------------
	// 1. Check OAuth error
	// ------------------------------------------------

	if oauthError := ctx.QueryParam("error"); oauthError != "" {
		return utils.Err(http.StatusUnauthorized, "Google authentication failed: "+oauthError)
	}

	// ------------------------------------------------
	// 2. Validate state
	// ------------------------------------------------

	stateCookie, err := ctx.Cookie("oauth_state")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "missing OAuth state")
	}

	state := ctx.QueryParam("state")

	if state == "" || state != stateCookie.Value {
		return utils.Err(http.StatusBadRequest, "invalid OAuth state")
	}

	// Delete state cookie after validation.
	utils.DeleteCookie(ctx, "oauth_state")

	// ------------------------------------------------
	// 3. Get PKCE verifier
	// ------------------------------------------------

	verifierCookie, err := ctx.Cookie("oauth_verifier")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "missing PKCE verifier")
	}

	verifier := verifierCookie.Value

	// Delete verifier after use.
	utils.DeleteCookie(ctx, "oauth_verifier")

	// ------------------------------------------------
	// 4. Get nonce cookie
	// ------------------------------------------------

	nonceCookie, err := ctx.Cookie("oauth_nonce")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "missing PKCE verifier")
	}

	expectedNonce := nonceCookie.Value

	// // Delete oauth_nonce after use.
	utils.DeleteCookie(ctx, "oauth_nonce")

	// ------------------------------------------------
	// 4. Get authorization code
	// ------------------------------------------------

	code := ctx.QueryParam("code")

	if code == "" {
		return utils.Err(http.StatusBadRequest, "missing authorization code")
	}

	// ------------------------------------------------
	// 5. Exchange code for Google token
	// ------------------------------------------------

	fullToken, err := c.oauth.Exchange(
		ctxr,
		code,
		oauth2.VerifierOption(verifier),
	)

	if err != nil {
		return utils.Err(http.StatusUnauthorized, "failed to exchange authorization code: "+err.Error())
	}

	rawIDToken, ok := fullToken.Extra("id_token").(string)
	if !ok {
		return utils.Err(http.StatusUnauthorized, "Missing id_token in Google response")
	}

	verifiedGoogleIdToken, err := idtoken.Validate(ctxr, rawIDToken, c.googleClientID)
	if err != nil {
		return utils.Err(http.StatusUnauthorized, "Failed to validate Google ID token: "+err.Error())
	}

	utils.DebugLogger.Printf("ValidatedIdToken: %+v\n", verifiedGoogleIdToken)

	if expectedNonce != verifiedGoogleIdToken.Claims["nonce"] {
		return utils.Err(http.StatusUnauthorized, "Nonce value didn't match!")
	}

	// ------------------------------------------------
	// 6. Use access token to get Google user
	// ------------------------------------------------

	client := c.oauth.Client(ctxr, fullToken)

	// Initialize Google OAuth2 service using official SDK
	oauth2Service, err := oauth2api.NewService(ctxr, option.WithHTTPClient(client))
	if err != nil {
		return utils.ErrInternal("Failed to initialize Google OAuth2 service", err)
	}

	// Fetch Google user profile
	userInfo, err := oauth2Service.Userinfo.V2.Me.Get().Do()
	if err != nil {
		return utils.ErrInternal("Failed to get Google user", err)
	}

	googleUser := GoogleUser{
		ID:            userInfo.Id,
		Email:         userInfo.Email,
		VerifiedEmail: userInfo.VerifiedEmail != nil && *userInfo.VerifiedEmail,
		Name:          userInfo.Name,
		Picture:       userInfo.Picture,
	}

	// ------------------------------------------------
	// 7. Verify Google account
	// ------------------------------------------------

	if googleUser.ID == "" || googleUser.Email == "" {
		return utils.Err(http.StatusUnauthorized, "Invalid Google account")
	}

	if !googleUser.VerifiedEmail {
		return utils.Err(http.StatusUnauthorized, "Google email is not verified")
	}

	// ------------------------------------------------
	// 8. Find/Create user in your database
	// ------------------------------------------------

	if errs := c.service.FindOrCreateUser(ctxr, &googleUser); errs != nil {
		return utils.ErrInternal("failed to provision account", errs)
	}

	jwtToken, err := c.tokens.GenerateOAuthToken(googleUser.ID, "google")
	if err != nil {
		return utils.ErrInternal("failed to create OAuth session", err)
	}
	// 5. Set Cookie with jwtToken
	utils.SetCookie(ctx, "access_token", jwtToken, 3600)

	return ctx.Redirect(http.StatusTemporaryRedirect, c.frontendURL+"/complete-oauth-profile")
}

func (c *Controller) SetOauthMobileAndPassword(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.CompleteOauthRequest](ctx, middlewares.ValidatedBodyKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	claims, err := echo.ContextGet[*utils.OauthClaims](ctx, middlewares.UserContextKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	err = c.service.SetOauthMobileAndPassword(ctx.Request().Context(), claims.Provider_id, claims.Provider, payload)
	if err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	utils.DeleteCookie(ctx, "access_token")

	return utils.SendResponse[any](ctx, http.StatusOK, "Account setup completed successfully", nil)
}

func (c *Controller) VerifyOTP(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.VerifyOTP](ctx, middlewares.ValidatedBodyKey)
	platform, _ := echo.ContextGet[string](ctx, middlewares.PlatformContextKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid payload!")
	}

	token, refreshToken, user, err := c.service.VerifyOTP(ctx.Request().Context(), payload)

	if err != nil {
		return utils.Err(http.StatusForbidden, err.Error())
	}

	utils.DebugLogger.Printf("From verify-otp:-[platform]:%s", platform)

	if platform == "mobile" {

		res := dto.UserResponseWithToken{
			User:         *user,
			Token:        token,
			RefreshToken: refreshToken,
		}

		return utils.SendResponse[*dto.UserResponseWithToken](ctx, http.StatusAccepted, "Logged In successfully!", &res)

	}
	utils.SetCookie(ctx, "access_token", token, int(utils.AccessTokenLifetime/time.Second))
	utils.SetRefreshTokenCookie(ctx, refreshToken, int(refreshTokenLifetime/time.Second))

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusAccepted, "Logged In successfully!", user)

}

func (c *Controller) Refresh(ctx *echo.Context) error {
	utils.DebugLogger.Printf("Refresh route hit!")
	var payload dto.RefreshRequest
	if err := ctx.Bind(&payload); err != nil {
		return utils.Err(http.StatusBadRequest, "invalid refresh request")
	}
	rawToken := payload.RefreshToken
	utils.DebugLogger.Printf("rawToken From mobile:%s ", rawToken)
	platform, _ := echo.ContextGet[string](ctx, middlewares.PlatformContextKey)
	utils.DebugLogger.Printf("Platform:%s ", platform)
	if platform != "mobile" {
		if cookie, err := ctx.Cookie("refresh_token"); err == nil && cookie.Value != "" {
			rawToken = cookie.Value
		}
	}

	accessToken, nextRefreshToken, user, err := c.service.Refresh(ctx.Request().Context(), rawToken)
	if err != nil {
		utils.DeleteRefreshTokenCookie(ctx)
		return utils.Err(http.StatusUnauthorized, "invalid or expired refresh token")
	}

	if platform == "mobile" {
		return utils.SendResponse[*dto.UserResponseWithToken](ctx, http.StatusOK, "Token refreshed successfully", &dto.UserResponseWithToken{User: *user, Token: accessToken, RefreshToken: nextRefreshToken})
	}

	utils.SetCookie(ctx, "access_token", accessToken, int(utils.AccessTokenLifetime/time.Second))
	utils.SetRefreshTokenCookie(ctx, nextRefreshToken, int(refreshTokenLifetime/time.Second))
	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusOK, "Token refreshed successfully", user)
}

func (c *Controller) ResendOTP(ctx *echo.Context) error {

	payload, err := echo.ContextGet[dto.ResendOTPRequest](ctx, middlewares.ValidatedBodyKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	err = c.service.ResendOTP(ctx.Request().Context(), payload.Email)

	if err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[any](ctx, http.StatusOK, "OTP resent successfully to your email", nil)
}

func (c *Controller) Logout(ctx *echo.Context) error {
	if cookie, err := ctx.Cookie("refresh_token"); err == nil {
		_ = c.service.RevokeRefreshToken(ctx.Request().Context(), cookie.Value)
	}
	utils.DeleteCookie(ctx, "access_token")
	utils.DeleteRefreshTokenCookie(ctx)

	return utils.SendResponse[*dto.LoginResponse](ctx, http.StatusOK, "Logged out successfully!", nil)
}

func (c *Controller) SendPasswordResetEmail(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.SendPasswordResetLinkRequest](ctx, middlewares.ValidatedBodyKey)
	platform, _ := echo.ContextGet[string](ctx, middlewares.PlatformContextKey)

	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	if err := c.service.SendForgetPasswordLink(ctx.Request().Context(), payload, platform); err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[any](ctx, http.StatusOK, "Password reset email sent successfully! Please check your inbox and spam folder.", nil)
}

func (c *Controller) CheckToken(ctx *echo.Context) error {
	token := ctx.QueryParam("token")
	platform := ctx.QueryParam("platform")

	encodedToken := url.QueryEscape(token)

	switch platform {
	case "web":
		redirectURL := fmt.Sprintf(
			"%s/reset-password?token=%s",
			config.AppConfig.FRONTEND_URL,
			encodedToken,
		)

		return ctx.Redirect(http.StatusFound, redirectURL)

	case "mobile":
		redirectURL := fmt.Sprintf(
			"%s://reset-password?token=%s",
			config.AppConfig.EXPO_SCHEME,
			encodedToken,
		)

		return ctx.Redirect(http.StatusFound, redirectURL)

	default:
		return utils.Err(
			http.StatusBadRequest,
			"invalid platform",
		)
	}
}

func (c *Controller) ResendPasswordResetEmail(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.ResendForgetPasswordRequest](ctx, middlewares.ValidatedBodyKey)
	platform, _ := echo.ContextGet[string](ctx, middlewares.PlatformContextKey)

	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	if err := c.service.ResendForgetPassword(ctx.Request().Context(), payload, platform); err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[any](ctx, http.StatusOK, "Password reset email resent successfully! Please check your inbox and spam folder.", nil)
}

func (c *Controller) SetNewPassword(ctx *echo.Context) error {
	payload, err := echo.ContextGet[dto.SetNewPassword](ctx, middlewares.ValidatedBodyKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}
	_ = fmt.Sprintf("Payload:%+v", payload)

	if err := c.service.SetNewPassword(ctx.Request().Context(), payload); err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[any](ctx, http.StatusOK, "Password reset has been done successfully!", nil)
}

func (c *Controller) GetProfile(ctx *echo.Context) error {
	claims, err := echo.ContextGet[*utils.Claims](ctx, middlewares.UserContextKey)
	if err != nil || claims == nil {
		return utils.Err(http.StatusUnauthorized, "Unauthorized access")
	}

	userResponse, err := c.service.GetProfile(ctx.Request().Context(), claims.UserEmail)
	if err != nil {
		return utils.Err(http.StatusNotFound, err.Error())
	}

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusOK, "User profile retrieved successfully!", userResponse)
}

func (c *Controller) UpdateProfile(ctx *echo.Context) error {
	claims, err := echo.ContextGet[*utils.Claims](ctx, middlewares.UserContextKey)
	if err != nil || claims == nil {
		return utils.Err(http.StatusUnauthorized, "Unauthorized access")
	}

	payload, err := echo.ContextGet[dto.UpdateProfileRequest](ctx, middlewares.ValidatedBodyKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	userResponse, err := c.service.UpdateProfile(ctx.Request().Context(), claims.UserID, payload)
	if err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusOK, "User profile updated successfully!", userResponse)
}

func (c *Controller) ChangePassword(ctx *echo.Context) error {
	claims, err := echo.ContextGet[*utils.Claims](ctx, middlewares.UserContextKey)
	if err != nil {
		return utils.Err(http.StatusUnauthorized, "Unauthorized access")
	}

	payload, err := echo.ContextGet[dto.ChangePasswordRequest](ctx, middlewares.ValidatedBodyKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	if err := c.service.ChangePassword(ctx.Request().Context(), claims.UserEmail, payload); err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[any](ctx, http.StatusOK, "Password changed successfully!", nil)
}

func (c *Controller) GetAllUsers(ctx *echo.Context) error {
	// Generic query helpers: type-safe parsing with defaults, no strconv noise.
	page, _ := echo.QueryParamOr[int](ctx, "page", 1)
	limit, _ := echo.QueryParamOr[int](ctx, "limit", 20)
	search, _ := echo.QueryParamOr[string](ctx, "search", "")
	role, _ := echo.QueryParamOr[string](ctx, "role", "")

	res, err := c.service.GetAllUsers(ctx.Request().Context(), page, limit, search, role)
	if err != nil {
		return utils.ErrInternal("could not list users", err)
	}

	return utils.SendResponse[*dto.PaginatedUserResponse](ctx, http.StatusOK, "Users retrieved successfully!", res)
}

func (c *Controller) GetUserByID(ctx *echo.Context) error {
	// Generic path helper: parses the :id path value straight into uint64.
	userID, err := echo.PathParam[uint64](ctx, "id")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid user ID parameter")
	}

	userResponse, err := c.service.GetUserByID(ctx.Request().Context(), uint(userID))
	if err != nil {
		return utils.Err(http.StatusNotFound, err.Error())
	}

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusOK, "User retrieved successfully!", userResponse)
}

func (c *Controller) UpdateUser(ctx *echo.Context) error {
	userID, err := echo.PathParam[uint64](ctx, "id")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid user ID parameter")
	}

	payload, err := echo.ContextGet[dto.UpdateUserRequest](ctx, middlewares.ValidatedBodyKey)
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid request payload")
	}

	userResponse, err := c.service.UpdateUser(ctx.Request().Context(), uint(userID), payload)
	if err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusOK, "User updated successfully!", userResponse)
}

func (c *Controller) DeleteUser(ctx *echo.Context) error {
	userID, err := echo.PathParam[uint64](ctx, "id")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid user ID parameter")
	}

	if err := c.service.DeleteUser(ctx.Request().Context(), uint(userID)); err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[any](ctx, http.StatusOK, "User deleted successfully!", nil)
}

// BlockUser disables a user's access without deleting the account.
//
// It is a thin HTTP wrapper: parse the :id path value, hand it to the service
// use-case, and render the standard envelope. Admin role is enforced by the
// guard registered in RegisterRoutes, so this handler never checks roles itself.
func (c *Controller) BlockUser(ctx *echo.Context) error {
	userID, err := echo.PathParam[uint64](ctx, "id")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid user ID parameter")
	}

	userResponse, err := c.service.BlockUser(ctx.Request().Context(), uint(userID))
	if err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusOK, "User blocked successfully!", userResponse)
}

// UnblockUser restores a previously blocked user's access.
func (c *Controller) UnblockUser(ctx *echo.Context) error {
	userID, err := echo.PathParam[uint64](ctx, "id")
	if err != nil {
		return utils.Err(http.StatusBadRequest, "Invalid user ID parameter")
	}

	userResponse, err := c.service.UnblockUser(ctx.Request().Context(), uint(userID))
	if err != nil {
		return utils.Err(http.StatusBadRequest, err.Error())
	}

	return utils.SendResponse[*dto.UserResponse](ctx, http.StatusOK, "User unblocked successfully!", userResponse)
}
