package dto

type Role string

const (
	User    Role = "user"
	Manager Role = "manager"
	Admin   Role = "admin"
)

type RegisterRequest struct {
	Username        string `json:"username" validate:"required,min=1,max=200"`
	Email           string `json:"email" validate:"required,email_rfc5322"`
	Mobile          string `json:"mobile" validate:"required,len=11"`
	Password        string `json:"password" validate:"required,min=6,max=200"`
	ConfirmPassword string `json:"confirm_password,omitempty" validate:"omitempty,eqfield=Password"`
}

type CompleteOauthRequest struct {
	Mobile   string `json:"mobile" validate:"required,len=11"`
	Password string `json:"password" validate:"required,min=6,max=200"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email_rfc5322"`
	Password string `json:"password" validate:"required"`
}

type UpdateProfileRequest struct {
	Username string `json:"username,omitempty" validate:"omitempty,min=1,max=200"`
	Email    string `json:"email,omitempty" validate:"omitempty,email"`
	Mobile   string `json:"mobile,omitempty" validate:"omitempty,len=11"`
}

type UpdateUserRequest struct {
	Username    string `json:"username,omitempty" validate:"omitempty,min=1,max=200"`
	Email       string `json:"email,omitempty" validate:"omitempty,email"`
	Mobile      string `json:"mobile,omitempty" validate:"omitempty,len=11"`
	Role        Role   `json:"role,omitempty" validate:"omitempty,oneof=admin user manager"`
	IsActivated *bool  `json:"is_activated,omitempty"`
	IsBlocked   *bool  `json:"is_blocked,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6,max=200"`
}

type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

type SetNewPassword struct {
	Password string
	Token    string
}

type VerifyOTP struct {
	Email string `json:"email" validate:"required"`
	OTP   string `json:"otp" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

type ResendActivationRequest struct {
	Email string `json:"email" validate:"required,email_rfc5322"`
}

type SendPasswordResetLinkRequest struct {
	Email string `json:"email" validate:"required,email_rfc5322"`
}

type ResendOTPRequest struct {
	Email string `json:"email" validate:"required,email_rfc5322"`
}

type ResendForgetPasswordRequest struct {
	Email string `json:"email" validate:"required,email_rfc5322"`
}

type GetProfile struct {
	Email string `json:"email" validate:"required,email_rfc5322"`
}
