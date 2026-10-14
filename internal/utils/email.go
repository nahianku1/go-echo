package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"net-http/internal/config"

	"gopkg.in/mail.v2"
)

type SMTPMailer struct {
	config config.Config
}

type verificationEmailData struct {
	AppName         string
	Name            string
	ExpiresIn       string
	VerificationURL string
	Year            int
}

type passwordResetEmailData struct {
	AppName   string
	Name      string
	ExpiresIn string
	ResetURL  string
	Year      int
}

type OTPEmailData struct {
	AppName string
	Name    string
	OtpUrl  string
	OTP     string
	Year    int
}

func GenerateRandomToken() (string, error) {
	data := make([]byte, 32)

	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	return hex.EncodeToString(data), nil
}

func NewSMTPMailer(cfg config.Config) (*SMTPMailer, error) {
	return &SMTPMailer{
		config: cfg,
	}, nil
}

func (m *SMTPMailer) SendActivationEmail(
	ctx context.Context,
	to string,
	name string,
	token string,
	platform string,
) error {
	verifyURL := fmt.Sprintf(
		"%s/api/auth/verify-email?token=%s&platform=%s",
		m.config.BACKEND_URL,
		token, platform,
	)

	fmt.Println("URL:", verifyURL)

	body, err := RenderTemplate(
		"./internal/templates/verify-email.html",
		verificationEmailData{
			AppName:         m.config.MAIL_BRAND,
			Name:            name,
			VerificationURL: verifyURL,
			ExpiresIn:       "5 minutes",
			Year:            time.Now().Year(),
		},
	)

	if err != nil {
		return err
	}

	msg := mail.NewMessage()
	msg.SetHeader("From", fmt.Sprintf("%s <%s>", m.config.MAIL_BRAND, m.config.SMTP_FROM))
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Verify your email")
	msg.SetBody("text/html", body)

	SMTP_PORT, _ := strconv.Atoi(m.config.SMTP_PORT)

	d := mail.NewDialer(m.config.SMTP_HOST, SMTP_PORT, m.config.SMTP_USER, m.config.SMTP_PASSWORD)

	if err := d.DialAndSend(msg); err != nil {
		return fmt.Errorf("Send verification email failed: %w", err)
	}

	fmt.Printf("Activation email sent to: %s\n", to)

	return nil
}

func (m *SMTPMailer) SendPasswordResetLink(
	ctx context.Context,
	email string,
	name string,
	token string,
	platform string,
) error {
	var resetURL string

	resetURL = fmt.Sprintf(
		"%s/api/auth/check-token?token=%s&platform=%s",
		m.config.BACKEND_URL,
		token, platform,
	)

	body, err := RenderTemplate(
		"./internal/templates/forget-password.html",
		passwordResetEmailData{
			AppName:   m.config.MAIL_BRAND,
			Name:      name,
			ResetURL:  resetURL,
			ExpiresIn: "5 minutes",
			Year:      time.Now().Year(),
		},
	)

	if err != nil {
		return err
	}

	fromAddr := m.config.SMTP_FROM

	msg := mail.NewMessage()
	msg.SetHeader("From", fmt.Sprintf("%s <%s>", m.config.MAIL_BRAND, fromAddr))
	msg.SetHeader("To", email)
	msg.SetHeader("Subject", "Verify your email")
	msg.SetBody("text/html", body)

	port, _ := strconv.Atoi(m.config.SMTP_PORT)

	d := mail.NewDialer(m.config.SMTP_HOST, port, m.config.SMTP_USER, m.config.SMTP_PASSWORD)

	if err := d.DialAndSend(msg); err != nil {
		return fmt.Errorf("Send verification email failed: %w", err)
	}

	fmt.Printf("Password reset link sent to: %s\n", email)

	return nil
}

func (m *SMTPMailer) SendOTP(
	ctx context.Context,
	email string,
	name string,
	otp string,
) error {

	body, err := RenderTemplate(
		"./internal/templates/send-otp.html",
		OTPEmailData{
			AppName: m.config.MAIL_BRAND,
			Name:    name,
			OtpUrl:  fmt.Sprintf("%s/otp-verify", m.config.FRONTEND_URL),
			OTP:     otp,
			Year:    time.Now().Year(),
		},
	)

	if err != nil {
		return err
	}

	msg := mail.NewMessage()
	msg.SetHeader("From", fmt.Sprintf("%s <%s>", m.config.MAIL_BRAND, m.config.SMTP_FROM))
	msg.SetHeader("To", email)
	msg.SetHeader("Subject", "OTP Verification")
	msg.SetBody("text/html", body)

	port, _ := strconv.Atoi(m.config.SMTP_PORT)

	d := mail.NewDialer(m.config.SMTP_HOST, port, m.config.SMTP_USER, m.config.SMTP_PASSWORD)

	if err := d.DialAndSend(msg); err != nil {
		return fmt.Errorf("Send verification email failed: %w", err)
	}

	fmt.Printf("An OTP sent to: %s\n", email)

	return nil
}
