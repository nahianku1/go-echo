package auth

import (
	"time"
	"gorm.io/gorm"
)

type Role string

const (
	Admin    Role = "admin"
	UserRole Role = "user"
	Manager  Role = "manager"
)

// User represents the GORM database entity stored in PostgreSQL
type User struct {
	gorm.Model
	Username                  string `gorm:"type:varchar(100);not null"`
	OauthId                   string `gorm:"type:varchar(100);default:null;uniqueIndex"`
	Email                     string `gorm:"type:varchar(100);uniqueIndex;not null"`
	Mobile                    string `gorm:"type:varchar(20);uniqueIndex;default:null"`
	ProfilePic                string `gorm:"type:varchar(255);default:null"`
	Password                  string `gorm:"type:varchar(255);default:null"`
	Role                      Role   `gorm:"type:varchar(20);default:'user';not null"`
	IsActivated               bool   `gorm:"default:false;not null"`
	IsBlocked                 bool   `gorm:"default:false;not null"`
	ActivationToken           string `gorm:"type:varchar(255);index"`
	NewPasswordToken          string `gorm:"type:varchar(255);index"`
	OtpToken                  string `gorm:"type:varchar(255);index"`
	ActivationTokenExpiresAt  *time.Time
	NewPasswordTokenExpiresAt *time.Time
	OtpTokenExpiresAt         *time.Time
}

type AuthAccount struct {
	gorm.Model

	UserID uint `gorm:"not null;index"`

	Provider   string `gorm:"type:varchar(30);not null"`
	ProviderID string `gorm:"type:varchar(255);not null"`

	User User `gorm:"foreignKey:UserID"`
}
