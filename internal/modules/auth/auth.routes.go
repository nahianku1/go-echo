package auth

import (
	"net-http/internal/middlewares"
	"net-http/internal/modules/auth/dto"

	"github.com/labstack/echo/v5"
)

// RegisterRoutes mounts the auth endpoints on the given echo instance.
//
// It is a method on the controller and takes the auth middleware as an argument
// rather than reaching for a process-wide AuthController / package-level guard.
// Route wiring is composition, so it happens once, explicitly, from main: the
// same controller can be registered on a different echo instance (or with a
// different guard) without any global state changing underneath it.
func (c *Controller) RegisterRoutes(e *echo.Echo, auth *middlewares.Auth) {
	// Public auth routes
	e.POST("/api/auth/register", c.Register, middlewares.ValidateBody[dto.RegisterRequest]())
	e.GET("/api/auth/verify-email", c.VerifyEmail)
	e.POST("/api/auth/resend-activation", c.ResendActivationEmail, middlewares.ValidateBody[dto.ResendActivationRequest]())

	// Public Login routes
	e.POST("/api/auth/login", c.Login, middlewares.ValidateBody[dto.LoginRequest]())
	e.POST("/api/auth/verify-otp", c.VerifyOTP, middlewares.ValidateBody[dto.VerifyOTP]())
	e.POST("/api/auth/resend-otp", c.ResendOTP, middlewares.ValidateBody[dto.ResendOTPRequest]())
	e.POST("/api/auth/refresh", c.Refresh)

	// Public Google Oauth login routes
	e.GET("/api/auth/google_login", c.GoogleLogin)
	e.GET("/api/auth/google/callback", c.GoogleCallback)
	e.POST("/api/auth/complete-oauth-profile", c.SetOauthMobileAndPassword, auth.Require(), middlewares.ValidateBody[dto.CompleteOauthRequest]())

	// Public Logout route
	e.POST("/api/auth/logout", c.Logout)

	// Public password operation routes
	e.POST("/api/auth/forget-password", c.SendPasswordResetEmail, middlewares.ValidateBody[dto.SendPasswordResetLinkRequest]())
	e.GET("/api/auth/check-token", c.CheckToken)
	e.POST("/api/auth/resend-forget-password", c.ResendPasswordResetEmail, middlewares.ValidateBody[dto.ResendForgetPasswordRequest]())
	e.POST("/api/auth/set-password", c.SetNewPassword, middlewares.ValidateBody[dto.SetNewPassword]())

	// Protected Profile routes (Protected by the injected auth middleware)
	e.GET("/api/auth/me", c.GetProfile, auth.Require("user", "admin", "manager"))
	e.PUT("/api/auth/profile", c.UpdateProfile, auth.Require("user", "admin", "manager"), middlewares.ValidateBody[dto.UpdateProfileRequest]())
	e.PATCH("/api/auth/change-password", c.ChangePassword, auth.Require("user", "admin", "manager"), middlewares.ValidateBody[dto.ChangePasswordRequest]())

	// User management routes (Protected, Admin Only)
	//------------------------------------------------------------------------------------------------
	e.GET("/api/users", c.GetAllUsers, auth.Require("admin"))
	e.GET("/api/users/:id", c.GetUserByID, auth.Require("admin"))
	e.PUT("/api/users/:id", c.UpdateUser, auth.Require("admin"), middlewares.ValidateBody[dto.UpdateUserRequest]())

	// Delete user (Protected & Admin Role requirement)
	e.DELETE("/api/users/:id", c.DeleteUser, auth.Require("admin"))

	// Block / unblock a user (Protected & Admin Role requirement)
	e.PATCH("/api/users/:id/block", c.BlockUser, auth.Require("admin"))
	e.PATCH("/api/users/:id/unblock", c.UnblockUser, auth.Require("admin"))
	//-----------------------------------------------------------------------------------------------------------
}
