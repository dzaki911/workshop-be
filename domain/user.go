package domain

import (
	"context"
	"time"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleBPH   Role = "bph"
	RoleStaff Role = "staff"
)

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	Position     *string   `json:"position,omitempty"`
	Ministry     *string   `json:"ministry,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RoleType mengembalikan tipe aspek yang berlaku untuk user ini.
// Admin tidak dinilai sehingga ok bernilai false.
func (u User) RoleType() (RoleType, bool) {
	switch u.Role {
	case RoleBPH:
		return RoleTypeBPH, true
	case RoleStaff:
		return RoleTypeStaff, true
	}
	return "", false
}

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	// ListFunctionaries mengembalikan user ber-role bph/staff; role nil = keduanya.
	ListFunctionaries(ctx context.Context, role *Role) ([]User, error)
}

type UserUsecase interface {
	ListFunctionaries(ctx context.Context, role string) ([]User, error)
}
