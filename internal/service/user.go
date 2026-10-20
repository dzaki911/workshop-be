package service

import (
	"context"
	"strings"

	"workshop-be/domain"
)

type userService struct{ users domain.UserRepository }

func NewUserService(users domain.UserRepository) domain.UserUsecase {
	return &userService{users: users}
}

// ListFunctionaries mengembalikan BPH/staff; role kosong = keduanya.
func (s *userService) ListFunctionaries(ctx context.Context, role string) ([]domain.User, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return s.users.ListFunctionaries(ctx, nil)
	}
	r := domain.Role(role)
	if r != domain.RoleBPH && r != domain.RoleStaff {
		return nil, domain.ValidationErrors{{Field: "role", Message: "role harus bph atau staff"}}
	}
	return s.users.ListFunctionaries(ctx, &r)
}
