package service

import (
	"context"
	"strings"

	"workshop-be/domain"
)

type aspectService struct{ aspects domain.AspectRepository }

func NewAspectService(aspects domain.AspectRepository) domain.AspectUsecase {
	return &aspectService{aspects: aspects}
}

func (s *aspectService) List(ctx context.Context, roleType string) ([]domain.Aspect, error) {
	roleType = strings.ToLower(strings.TrimSpace(roleType))
	if roleType == "" {
		return s.aspects.List(ctx, nil)
	}
	rt := domain.RoleType(roleType)
	if !rt.Valid() {
		return nil, domain.ValidationErrors{{Field: "role_type", Message: "role_type harus bph atau staff"}}
	}
	return s.aspects.List(ctx, &rt)
}

func (s *aspectService) Get(ctx context.Context, id int64) (*domain.Aspect, error) {
	return s.aspects.GetByID(ctx, id)
}

func validateAspect(in domain.AspectInput) (domain.Aspect, error) {
	var errs domain.ValidationErrors
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		errs.Add("name", "Nama aspek wajib diisi")
	case len([]rune(name)) > 100:
		errs.Add("name", "Nama aspek maksimal 100 karakter")
	}
	rt := domain.RoleType(strings.ToLower(strings.TrimSpace(in.RoleType)))
	if !rt.Valid() {
		errs.Add("role_type", "role_type harus bph atau staff")
	}
	if len(errs) > 0 {
		return domain.Aspect{}, errs
	}
	return domain.Aspect{Name: name, Description: strings.TrimSpace(in.Description), RoleType: rt}, nil
}

func (s *aspectService) Create(ctx context.Context, in domain.AspectInput) (*domain.Aspect, error) {
	a, err := validateAspect(in)
	if err != nil {
		return nil, err
	}
	if err := s.aspects.Create(ctx, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *aspectService) Update(ctx context.Context, id int64, in domain.AspectInput) (*domain.Aspect, error) {
	next, err := validateAspect(in)
	if err != nil {
		return nil, err
	}
	cur, err := s.aspects.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Mengubah role_type aspek yang sudah dipakai akan merusak konsistensi raport tersimpan.
	if next.RoleType != cur.RoleType {
		used, err := s.aspects.IsUsed(ctx, id)
		if err != nil {
			return nil, err
		}
		if used {
			return nil, domain.ErrAspectInUse
		}
	}
	next.ID = id
	next.CreatedAt = cur.CreatedAt
	if err := s.aspects.Update(ctx, &next); err != nil {
		return nil, err
	}
	return &next, nil
}

func (s *aspectService) Delete(ctx context.Context, id int64) error {
	if _, err := s.aspects.GetByID(ctx, id); err != nil {
		return err
	}
	used, err := s.aspects.IsUsed(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return domain.ErrAspectInUse
	}
	return s.aspects.Delete(ctx, id)
}
