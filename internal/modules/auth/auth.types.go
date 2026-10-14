package auth

import "context"

type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}


type Mailer interface {
	SendActivationEmail(ctx context.Context, to, name, token string,platform string) error
	SendOTP(ctx context.Context, email, name, otp string) error
	SendPasswordResetLink(ctx context.Context, email, name, token string,plat string) error
}