package auth

import "context"

// Mailer is the seam between the auth use-cases and the way email actually
// leaves the process.
//
// Before this interface existed the service constructed
// utils.NewSMTPMailer(config.AppConfig) inside every goroutine, which welded the
// business logic to one concrete transport, to the global config and to a live
// network call. Depending on an abstraction instead lets the service run against
// an in-memory fake (no SMTP credentials, no network, no flakiness) and lets the
// transport be replaced without touching a single use-case. It is satisfied by
// *utils.SMTPMailer as-is.
type Mailer interface {
	SendActivationEmail(ctx context.Context, to, name, token string) error
	SendOTP(ctx context.Context, email, name, otp string) error
	SendPasswordResetLink(ctx context.Context, email, name, token string) error
}
