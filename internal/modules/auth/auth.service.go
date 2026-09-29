package auth

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"net-http/internal/modules/auth/dto"
	"net-http/internal/utils"
	"time"
)

// Service holds the auth use-cases.
//
// Its collaborators are injected through NewService instead of being reached for
// as package-level globals (database.DB, config.AppConfig, utils.NewSMTPMailer,
// utils.GenerateJWTToken). That inversion is what makes the service testable: a
// test can pass a fake Mailer/TokenService plus a test *gorm.DB, and nothing in
// this file needs to know how those were constructed.
type Service struct {
	db     *gorm.DB
	mailer Mailer
	tokens utils.TokenService
}

// NewService wires the auth use-cases to a database, an email transport and a
// token signer. Callers depend on the abstractions, never on concrete types.
func NewService(db *gorm.DB, mailer Mailer, tokens utils.TokenService) *Service {
	return &Service{db: db, mailer: mailer, tokens: tokens}
}

func (s *Service) Register(ctx context.Context, payload dto.RegisterRequest) (*dto.UserResponse, error) {
	// Check if user already exists with the same email
	var existingUser User
	err := s.db.WithContext(ctx).Where("email = ?", payload.Email).First(&existingUser).Error
	if err == nil {
		return nil, errors.New("user with this email already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database query error: %w", err)
	}

	// Check if user already exists with the same mobile
	if payload.Mobile != "" {
		var existingMobileUser User
		err = s.db.WithContext(ctx).Where("mobile = ?", payload.Mobile).First(&existingMobileUser).Error
		if err == nil {
			return nil, errors.New("User with this mobile already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("database query error: %w", err)
		}
	}

	hashedPassword := utils.CreatePasswordHash(payload.Password)

	activationToken, err := utils.GenerateRandomToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate activation token: %w", err)
	}

	expiresAt := time.Now().Add(5 * time.Minute)

	user := User{
		Username:                 payload.Username,
		Email:                    payload.Email,
		Mobile:                   payload.Mobile,
		Password:                 hashedPassword,
		Role:                     UserRole,
		IsActivated:              false,
		ActivationToken:          activationToken,
		ActivationTokenExpiresAt: &expiresAt,
	}

	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Send the activation email off the request path through the injected mailer.
	go func(email, username, token string) {
		if err := s.mailer.SendActivationEmail(context.Background(), email, username, token); err != nil {
			log.Printf("[ERROR] Failed to send activation email: %v", err)
		}
	}(user.Email, user.Username, activationToken)

	res := toUserResponse(&user)
	return &res, nil
}

func (s *Service) SetOauthMobileAndPassword(ctx context.Context, provider_id string, provider string, payload dto.CompleteOauthRequest) error {
	var account AuthAccount

	err := s.db.WithContext(ctx).
		Preload("User").
		Where("provider = ? AND provider_id = ?", provider, provider_id).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("Account not found")
	} else if err != nil {
		return fmt.Errorf("Database query error: %w", err)
	}

	if account.User.IsBlocked {
		return errors.New("your account has been blocked. Please contact an administrator")
	}

	if account.User.Password != "" && account.User.Mobile != "" {
		return errors.New("Password and Mobile already set!")
	}

	hashedPassword := utils.CreatePasswordHash(payload.Password)

	if err := s.db.WithContext(ctx).Model(&account.User).Updates(User{
		Mobile:   payload.Mobile,
		Password: hashedPassword,
		OauthId:  provider_id,
	}).Error; err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (s *Service) Login(ctx context.Context, payload dto.LoginRequest) error {
	var user User
	err := s.db.WithContext(ctx).Where("email = ?", payload.Email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("User doesn't exists!")
	} else if err != nil {
		return fmt.Errorf("Database query error: %w", err)
	}

	if user.IsBlocked {
		return errors.New("your account has been blocked. Please contact an administrator")
	}

	if !user.IsActivated {
		return errors.New("Please activate your account via the link sent to your email before logging in")
	}

	match, err := utils.VerifyPassword(payload.Password, user.Password)

	if err != nil || !match {
		return errors.New("Invalid email or password")
	}

	otp, err := utils.GenerateHexOTP()

	if err != nil {
		return errors.New("OTP Creation failed!")
	}

	// Send the OTP off the request path through the injected mailer.
	go func(email, username, otp string) {
		if err := s.mailer.SendOTP(context.Background(), email, username, otp); err != nil {
			log.Printf("[ERROR] Failed to send OTP email: %v", err)
		}
	}(user.Email, user.Username, otp)

	errs := s.db.WithContext(ctx).Model(&user).Updates(map[string]any{"otp_token": otp, "otp_token_expires_at": time.Now().Add(1 * time.Minute)}).Error
	if errs != nil {
		return errors.New("OTP data update failed!")
	}

	return nil
}

func (s *Service) FindOrCreateUser(ctx context.Context, googleUser *GoogleUser) error {
	var oauthAccount AuthAccount

	// 1. Find Google account
	result := s.db.WithContext(ctx).
		Where("provider = ? AND provider_id = ?", "google", googleUser.ID).
		First(&oauthAccount)

	if result.Error == nil {
		if oauthAccount.User.IsBlocked {
			return errors.New("your account has been blocked. Please contact an administrator")
		}
		// OAuth account already exists.
		return errors.New("User already exist!")
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	// 2. Google account doesn't exist.
	// Check whether a local user with this email already exists.
	var user User

	result = s.db.WithContext(ctx).
		Where("email = ?", googleUser.Email).
		First(&user)

	if result.Error == nil {
		if user.IsBlocked {
			return errors.New("your account has been blocked. Please contact an administrator")
		}
	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {

		// 3. Create new user.
		username := googleUser.Name
		email := googleUser.Email
		activated := googleUser.VerifiedEmail

		user = User{
			Username:    username,
			Email:       email,
			Password:    "",
			IsActivated: activated,
			ProfilePic:  googleUser.Picture,
		}

		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
			return err
		}

	} else if result.Error != nil {
		return result.Error
	}

	// 4. Connect Google account to the user.
	oauthAccount = AuthAccount{
		UserID:     user.ID,
		Provider:   "google",
		ProviderID: googleUser.ID,
	}

	if err := s.db.WithContext(ctx).Create(&oauthAccount).Error; err != nil {
		return err
	}

	return nil
}

func (s *Service) ResendOTP(ctx context.Context, email string) error {
	var user User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("user with this email does not exist")
	} else if err != nil {
		return fmt.Errorf("database query error: %w", err)
	}

	if user.IsBlocked {
		return errors.New("your account has been blocked. Please contact an administrator")
	}

	if !user.IsActivated {
		return errors.New("Please activate your account before requesting OTP")
	}

	otp, err := utils.GenerateHexOTP()
	if err != nil {
		return errors.New("OTP generation failed")
	}

	otpExpiresAt := time.Now().Add(1 * time.Minute)

	errs := s.db.WithContext(ctx).Model(&user).Updates(map[string]any{"otp_token": otp, "otp_token_expires_at": otpExpiresAt}).Error
	if errs != nil {
		return errors.New("failed to update OTP in database")
	}

	go func(email, username, otp string) {
		if err := s.mailer.SendOTP(context.Background(), email, username, otp); err != nil {
			log.Printf("[ERROR] Failed to send OTP email: %v", err)
		}
	}(user.Email, user.Username, otp)

	return nil
}

func (s *Service) VerifyOTP(ctx context.Context, payload dto.VerifyOTP) (string, *dto.UserResponse, error) {
	var user User

	err := s.db.WithContext(ctx).Where("otp_token = ?", payload.OTP).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, errors.New("OTP didn't matched!")
	} else if err != nil {
		return "", nil, errors.New("Database query error!")
	}
	if user.IsBlocked {
		return "", nil, errors.New("your account has been blocked. Please contact an administrator")
	}

	if user.OtpTokenExpiresAt != nil && time.Now().After(*user.OtpTokenExpiresAt) {
		return "", nil, errors.New("OTP has expired!")
	}

	token, err := s.tokens.GenerateAccessToken(user.ID, user.Username, string(user.Role), user.Email)

	if err != nil {
		return "", nil, errors.New("Token generation failed!")
	}

	errs := s.db.WithContext(ctx).Model(&user).Updates(map[string]any{"otp_token": nil, "otp_token_expires_at": nil}).Error
	if errs != nil {
		return "", nil, errors.New("OTP data update failed!")
	}

	res := toUserResponse(&user)

	return token, &res, nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	var user User
	err := s.db.WithContext(ctx).Where("activation_token = ?", token).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("invalid or expired activation token")
	} else if err != nil {
		return fmt.Errorf("database query error: %w", err)
	}

	if user.IsActivated {
		return errors.New("account is already activated")
	}

	if user.ActivationTokenExpiresAt != nil && time.Now().After(*user.ActivationTokenExpiresAt) {
		return errors.New("Activation token has expired. Please request a new activation email")
	}

	updates := map[string]any{
		"is_activated":                true,
		"activation_token_expires_at": nil,
	}

	if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to activate account: %w", err)
	}

	return nil
}

func (s *Service) ResendActivationEmail(ctx context.Context, email string) error {
	var user User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("user with this email does not exist")
	} else if err != nil {
		return fmt.Errorf("database query error: %w", err)
	}

	if user.IsActivated {
		return errors.New("account is already activated")
	}

	newToken, err := utils.GenerateRandomToken()
	if err != nil {
		return fmt.Errorf("failed to generate activation token: %w", err)
	}

	newExpiresAt := time.Now().Add(5 * time.Minute)

	updates := map[string]interface{}{
		"activation_token":            newToken,
		"activation_token_expires_at": newExpiresAt,
	}

	if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update activation token: %w", err)
	}

	go func(email, username, token string) {
		if err := s.mailer.SendActivationEmail(context.Background(), email, username, token); err != nil {
			log.Printf("[ERROR] Failed to send activation email: %v", err)
		}
	}(user.Email, user.Username, newToken)

	return nil
}

func (s *Service) ChangePassword(ctx context.Context, email string, payload dto.ChangePasswordRequest) error {
	var user User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("User not found")
		}
		return fmt.Errorf("Database query error: %w", err)
	}

	match, err := utils.VerifyPassword(payload.OldPassword, user.Password)
	if err != nil || !match {
		return errors.New("Old password didn't matched!")
	}

	newHashedPassword := utils.CreatePasswordHash(payload.NewPassword)
	if err := s.db.WithContext(ctx).Model(&user).Update("password", newHashedPassword).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func (s *Service) ForgetPassword(ctx context.Context, email string) error {
	var user User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("User with this email does not exist")
	} else if err != nil {
		return fmt.Errorf("Database query error: %w", err)
	}

	if user.IsBlocked {
		return errors.New("your account has been blocked. Please contact an administrator")
	}

	token, _ := utils.GenerateRandomToken()

	go func(email string, username string) {
		if err := s.mailer.SendPasswordResetLink(context.Background(), email, username, token); err != nil {
			log.Printf("[ERROR] Failed to send password reset email: %v", err)
		}
	}(user.Email, user.Username)

	updates := map[string]any{
		"new_password_token":            token,
		"new_password_token_expires_at": time.Now().Add(5 * time.Minute),
	}

	if err := s.db.WithContext(ctx).Model(&user).Debug().Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update password reset token: %w", err)
	}

	return nil
}

func (s *Service) ResendForgetPassword(ctx context.Context, email string) error {
	var user User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("User with this email does not exist")
	} else if err != nil {
		return fmt.Errorf("Database query error: %w", err)
	}

	if user.IsBlocked {
		return errors.New("your account has been blocked. Please contact an administrator")
	}

	if !user.IsActivated {
		return errors.New("Please activate your account before requesting password reset")
	}

	token, err := utils.GenerateRandomToken()
	if err != nil {
		return fmt.Errorf("failed to generate password reset token: %w", err)
	}

	updates := map[string]any{
		"new_password_token":            token,
		"new_password_token_expires_at": time.Now().Add(5 * time.Minute),
	}

	if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update password reset token: %w", err)
	}

	go func(email string, username string) {
		if err := s.mailer.SendPasswordResetLink(context.Background(), email, username, token); err != nil {
			log.Printf("[ERROR] Failed to send password reset email: %v", err)
		}
	}(user.Email, user.Username)

	return nil
}

func (s *Service) SetNewPassword(ctx context.Context, payload dto.SetNewPassword) error {
	var user User
	err := s.db.WithContext(ctx).Where("new_password_token = ?", payload.Token).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("User with this email does not exist")
	} else if err != nil {
		return fmt.Errorf("Database query error: %w", err)
	}

	if user.IsBlocked {
		return errors.New("your account has been blocked. Please contact an administrator")
	}

	if !user.IsActivated {
		return errors.New("Please activate your account via the link sent to your email before logging in")
	}

	if user.NewPasswordTokenExpiresAt != nil && time.Now().After(*user.NewPasswordTokenExpiresAt) {
		return errors.New("Password reset token has expired!")
	}

	match, _ := utils.VerifyPassword(payload.Password, user.Password)

	if match {
		return errors.New("Password can not be same as old one!")
	}

	hashedPassword := utils.CreatePasswordHash(payload.Password)

	if err := s.db.WithContext(ctx).Model(&user).Update("password", hashedPassword).Error; err != nil {
		utils.NewLogger("DATABASE").Printf("Errors: %+v", err)
		return errors.New("Password update failed!")

	}

	return nil
}

func (s *Service) GetProfile(ctx context.Context, email string) (*dto.UserResponse, error) {
	var user User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user profile: %w", err)
	}

	res := toUserResponse(&user)
	return &res, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uint, payload dto.UpdateProfileRequest) (*dto.UserResponse, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user profile: %w", err)
	}

	if payload.Email != "" && payload.Email != user.Email {
		var existingEmailUser User

		err := s.db.WithContext(ctx).
			Where("email = ? AND id != ?", payload.Email, user.ID).
			First(&existingEmailUser).Error

		if err == nil {
			return nil, errors.New("email is already in use")
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("database query error: %w", err)
		}
	}

	if payload.Mobile != "" && payload.Mobile != user.Mobile {
		var existingUser User

		err := s.db.WithContext(ctx).
			Where("mobile = ? AND id != ?", payload.Mobile, user.ID).
			First(&existingUser).Error

		if err == nil {
			return nil, errors.New("mobile number is already in use")
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("database query error: %w", err)
		}
	}

	updates := map[string]any{}
	if payload.Username != "" {
		updates["username"] = payload.Username
		user.Username = payload.Username
	}
	if payload.Email != "" {
		updates["email"] = payload.Email
		user.Email = payload.Email
	}
	if payload.Mobile != "" {
		updates["mobile"] = payload.Mobile
		user.Mobile = payload.Mobile
	}

	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update profile: %w", err)
		}
	}

	res := toUserResponse(&user)
	return &res, nil
}

func (s *Service) GetAllUsers(ctx context.Context, page, limit int, search, role string) (*dto.PaginatedUserResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := s.db.WithContext(ctx).Model(&User{})
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("username ILIKE ? OR email ILIKE ?", pattern, pattern)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	var users []User
	if err := query.Offset(offset).Limit(limit).Order("id asc").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	userResponses := make([]dto.UserResponse, len(users))
	for i, u := range users {
		userResponses[i] = toUserResponse(&u)
	}

	return &dto.PaginatedUserResponse{
		Users: userResponses,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *Service) GetUserByID(ctx context.Context, userID uint) (*dto.UserResponse, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	res := toUserResponse(&user)
	return &res, nil
}

func (s *Service) UpdateUser(ctx context.Context, userID uint, payload dto.UpdateUserRequest) (*dto.UserResponse, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	if payload.Email != "" && payload.Email != user.Email {
		var existingEmailUser User
		err := s.db.WithContext(ctx).
			Where("email = ? AND id != ?", payload.Email, user.ID).
			First(&existingEmailUser).Error

		if err == nil {
			return nil, errors.New("email is already in use")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("database query error: %w", err)
		}
	}

	if payload.Mobile != "" && payload.Mobile != user.Mobile {
		var existingMobileUser User
		err := s.db.WithContext(ctx).
			Where("mobile = ? AND id != ?", payload.Mobile, user.ID).
			First(&existingMobileUser).Error

		if err == nil {
			return nil, errors.New("mobile number is already in use")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("database query error: %w", err)
		}
	}

	updates := map[string]any{}
	if payload.Username != "" {
		updates["username"] = payload.Username
		user.Username = payload.Username
	}
	if payload.Email != "" {
		updates["email"] = payload.Email
		user.Email = payload.Email
	}
	if payload.Mobile != "" {
		updates["mobile"] = payload.Mobile
		user.Mobile = payload.Mobile
	}
	if payload.Role != "" {
		updates["role"] = payload.Role
		user.Role = Role(payload.Role)
	}
	if payload.IsActivated != nil {
		updates["is_activated"] = *payload.IsActivated
		user.IsActivated = *payload.IsActivated
	}
	if payload.IsBlocked != nil {
		updates["is_blocked"] = *payload.IsBlocked
		user.IsBlocked = *payload.IsBlocked
	}

	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	res := toUserResponse(&user)
	return &res, nil
}

func (s *Service) DeleteUser(ctx context.Context, userID uint) error {
	var user User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to fetch user for deletion: %w", err)
	}

	if err := s.db.WithContext(ctx).Delete(&user).Error; err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// BlockUser flips a user's IsBlocked flag on.
//
// Blocking is the soft alternative to DeleteUser: the row and its history stay
// intact, but every login/OTP/OAuth path already rejects IsBlocked users, so the
// account can no longer authenticate. The updated user is returned so the caller
// can render the new state without a follow-up read.
func (s *Service) BlockUser(ctx context.Context, userID uint) (*dto.UserResponse, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	if user.IsBlocked {
		return nil, errors.New("user is already blocked")
	}

	if err := s.db.WithContext(ctx).Model(&user).Update("is_blocked", true).Error; err != nil {
		return nil, fmt.Errorf("failed to block user: %w", err)
	}

	user.IsBlocked = true

	res := toUserResponse(&user)
	return &res, nil
}

// UnblockUser clears the IsBlocked flag, restoring the account's ability to
// authenticate.
func (s *Service) UnblockUser(ctx context.Context, userID uint) (*dto.UserResponse, error) {
	var user User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	if !user.IsBlocked {
		return nil, errors.New("user is not blocked")
	}

	if err := s.db.WithContext(ctx).Model(&user).Update("is_blocked", false).Error; err != nil {
		return nil, fmt.Errorf("failed to unblock user: %w", err)
	}

	user.IsBlocked = false

	res := toUserResponse(&user)
	return &res, nil
}

func toUserResponse(u *User) dto.UserResponse {
	return dto.UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Mobile:      u.Mobile,
		Role:        string(u.Role),
		IsActivated: u.IsActivated,
		IsBlocked:   u.IsBlocked,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
