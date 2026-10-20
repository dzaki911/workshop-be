// Package memory berisi implementasi repository in-memory dengan semantik yang sama
// seperti repository PostgreSQL (unique, in-use, cascade). Dipakai untuk test tanpa database.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"workshop-be/domain"
)

type Store struct {
	mu         sync.Mutex
	users      map[int64]domain.User
	aspects    map[int64]domain.Aspect
	reports    map[int64]domain.Report // Scores disimpan di dalam report
	nextUser   int64
	nextAspect int64
	nextReport int64
}

func NewStore() *Store {
	return &Store{
		users:   map[int64]domain.User{},
		aspects: map[int64]domain.Aspect{},
		reports: map[int64]domain.Report{},
	}
}

// AddUser menambah user (dipakai seed test); ID berurutan mulai 1.
func (s *Store) AddUser(name, email string, role domain.Role) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextUser++
	now := time.Now()
	s.users[s.nextUser] = domain.User{ID: s.nextUser, Name: name, Email: email, PasswordHash: "x", Role: role, CreatedAt: now, UpdatedAt: now}
	return s.nextUser
}

func (s *Store) AddAspect(name string, rt domain.RoleType) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAspect++
	now := time.Now()
	s.aspects[s.nextAspect] = domain.Aspect{ID: s.nextAspect, Name: name, RoleType: rt, CreatedAt: now, UpdatedAt: now}
	return s.nextAspect
}

func (s *Store) Users() domain.UserRepository     { return &userRepo{s} }
func (s *Store) Aspects() domain.AspectRepository { return &aspectRepo{s} }
func (s *Store) Reports() domain.ReportRepository { return &reportRepo{s} }

// ---- users ----

type userRepo struct{ s *Store }

func (r *userRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	u, ok := r.s.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (r *userRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, u := range r.s.users {
		if u.Email == email {
			u := u
			return &u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *userRepo) ListFunctionaries(_ context.Context, role *domain.Role) ([]domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := []domain.User{}
	for _, u := range r.s.users {
		if u.Role == domain.RoleAdmin || (role != nil && u.Role != *role) {
			continue
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// ---- aspects ----

type aspectRepo struct{ s *Store }

func (r *aspectRepo) dup(name string, rt domain.RoleType, exceptID int64) bool {
	for _, a := range r.s.aspects {
		if a.ID != exceptID && a.Name == name && a.RoleType == rt {
			return true
		}
	}
	return false
}

func (r *aspectRepo) List(_ context.Context, rt *domain.RoleType) ([]domain.Aspect, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := []domain.Aspect{}
	for _, a := range r.s.aspects {
		if rt == nil || a.RoleType == *rt {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RoleType != out[j].RoleType {
			return out[i].RoleType < out[j].RoleType
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *aspectRepo) GetByID(_ context.Context, id int64) (*domain.Aspect, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	a, ok := r.s.aspects[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &a, nil
}

func (r *aspectRepo) Create(_ context.Context, a *domain.Aspect) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if r.dup(a.Name, a.RoleType, 0) {
		return domain.ErrAspectDuplicate
	}
	r.s.nextAspect++
	now := time.Now()
	a.ID, a.CreatedAt, a.UpdatedAt = r.s.nextAspect, now, now
	r.s.aspects[a.ID] = *a
	return nil
}

func (r *aspectRepo) Update(_ context.Context, a *domain.Aspect) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.aspects[a.ID]; !ok {
		return domain.ErrNotFound
	}
	if r.dup(a.Name, a.RoleType, a.ID) {
		return domain.ErrAspectDuplicate
	}
	a.UpdatedAt = time.Now()
	r.s.aspects[a.ID] = *a
	return nil
}

func (r *aspectRepo) used(id int64) bool {
	for _, rep := range r.s.reports {
		for _, sc := range rep.Scores {
			if sc.AspectID == id {
				return true
			}
		}
	}
	return false
}

func (r *aspectRepo) Delete(_ context.Context, id int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.aspects[id]; !ok {
		return domain.ErrNotFound
	}
	if r.used(id) {
		return domain.ErrAspectInUse
	}
	delete(r.s.aspects, id)
	return nil
}

func (r *aspectRepo) IsUsed(_ context.Context, id int64) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return r.used(id), nil
}

// ---- reports ----

type reportRepo struct{ s *Store }

// hydrate mengisi field join (nama user, nama aspek) pada salinan report.
func (r *reportRepo) hydrate(rep domain.Report) domain.Report {
	u := r.s.users[rep.UserID]
	rep.UserName, rep.UserRole = u.Name, u.Role
	scores := make([]domain.ReportScore, len(rep.Scores))
	for i, sc := range rep.Scores {
		sc.AspectName = r.s.aspects[sc.AspectID].Name
		scores[i] = sc
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].AspectID < scores[j].AspectID })
	rep.Scores = scores
	return rep
}

func (r *reportRepo) List(_ context.Context) ([]domain.Report, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	out := []domain.Report{}
	for _, rep := range r.s.reports {
		h := r.hydrate(rep)
		h.Scores = nil // daftar tidak memuat nilai, sama seperti repository PostgreSQL
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (r *reportRepo) GetByID(_ context.Context, id int64) (*domain.Report, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	rep, ok := r.s.reports[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	h := r.hydrate(rep)
	return &h, nil
}

func (r *reportRepo) Create(_ context.Context, rep *domain.Report) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, ex := range r.s.reports {
		if ex.UserID == rep.UserID {
			return domain.ErrReportExists
		}
	}
	r.s.nextReport++
	now := time.Now()
	rep.ID, rep.CreatedAt, rep.UpdatedAt = r.s.nextReport, now, now
	r.s.reports[rep.ID] = *rep
	return nil
}

func (r *reportRepo) Update(_ context.Context, rep *domain.Report) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.reports[rep.ID]
	if !ok {
		return domain.ErrNotFound
	}
	cur.AverageScore = rep.AverageScore
	cur.OverallEvaluation = rep.OverallEvaluation
	cur.Scores = rep.Scores
	cur.UpdatedAt = time.Now()
	r.s.reports[rep.ID] = cur
	rep.UpdatedAt = cur.UpdatedAt
	return nil
}

func (r *reportRepo) Delete(_ context.Context, id int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.reports[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.s.reports, id)
	return nil
}
