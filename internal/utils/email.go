package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
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
) error {
	verifyURL := fmt.Sprintf(
		"http://%s/api/auth/verify-email?token=%s",
		net.JoinHostPort(m.config.HOST, m.config.PORT),
		token,
	)

	fmt.Println("URL:", verifyURL)

	body, err := RenderTemplate(
		"./internal/templates/verify-email.html",
		verificationEmailData{
			AppName:         "Mailtrap.io",
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
	msg.SetHeader("From", fmt.Sprintf("Mailtrap <%s>", m.config.SMTP_FROM))
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
) error {

	ResetURL := fmt.Sprintf(
		"http://localhost:5173/reset-password?token=%s",
		token,
	)

	body, err := RenderTemplate(
		"./internal/templates/forget-password.html",
		passwordResetEmailData{
			AppName:   "Mailtrap.io",
			Name:      name,
			ResetURL:  ResetURL,
			ExpiresIn: "5 minutes",
			Year:      time.Now().Year(),
		},
	)

	if err != nil {
		return err
	}

	fromAddr := m.config.SMTP_FROM

	msg := mail.NewMessage()
	msg.SetHeader("From", fmt.Sprintf("Mailtrap <%s>", fromAddr))
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
			AppName: "Mailtrap.io",
			Name:    name,
			OtpUrl:  "http://localhost:5173/otp-verify",
			OTP:     otp,
			Year:    time.Now().Year(),
		},
	)

	if err != nil {
		return err
	}

	fromAddr := m.config.SMTP_FROM

	msg := mail.NewMessage()
	msg.SetHeader("From", fmt.Sprintf("Mailtrap <%s>", fromAddr))
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
