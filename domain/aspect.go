package domain

import (
	"context"
	"time"
)

type RoleType string

const (
	RoleTypeBPH   RoleType = "bph"
	RoleTypeStaff RoleType = "staff"
)

func (r RoleType) Valid() bool { return r == RoleTypeBPH || r == RoleTypeStaff }

type Aspect struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	RoleType    RoleType  `json:"role_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AspectInput adalah payload create/update aspek.
type AspectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RoleType    string `json:"role_type"`
}

type AspectRepository interface {
	// List mengembalikan aspek; roleType nil = semua.
	List(ctx context.Context, roleType *RoleType) ([]Aspect, error)
	GetByID(ctx context.Context, id int64) (*Aspect, error)
	Create(ctx context.Context, a *Aspect) error
	Update(ctx context.Context, a *Aspect) error
	Delete(ctx context.Context, id int64) error
	// IsUsed true bila aspek sudah dipakai di report_scores.
	IsUsed(ctx context.Context, id int64) (bool, error)
}

type AspectUsecase interface {
	List(ctx context.Context, roleType string) ([]Aspect, error)
	Get(ctx context.Context, id int64) (*Aspect, error)
	Create(ctx context.Context, in AspectInput) (*Aspect, error)
	Update(ctx context.Context, id int64, in AspectInput) (*Aspect, error)
	Delete(ctx context.Context, id int64) error
}
