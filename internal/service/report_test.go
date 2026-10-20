package service

import (
	"context"
	"errors"
	"testing"

	"workshop-be/domain"
)

// ---- fakes ----

type fakeUsers struct{ byID map[int64]*domain.User }

func (f fakeUsers) GetByID(_ context.Context, id int64) (*domain.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}
func (f fakeUsers) GetByEmail(context.Context, string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (f fakeUsers) ListFunctionaries(context.Context, *domain.Role) ([]domain.User, error) {
	return nil, nil
}

type fakeAspects struct{ list []domain.Aspect }

func (f fakeAspects) List(_ context.Context, rt *domain.RoleType) ([]domain.Aspect, error) {
	var out []domain.Aspect
	for _, a := range f.list {
		if rt == nil || a.RoleType == *rt {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f fakeAspects) GetByID(context.Context, int64) (*domain.Aspect, error) {
	return nil, domain.ErrNotFound
}
func (f fakeAspects) Create(context.Context, *domain.Aspect) error { return nil }
func (f fakeAspects) Update(context.Context, *domain.Aspect) error { return nil }
func (f fakeAspects) Delete(context.Context, int64) error          { return nil }
func (f fakeAspects) IsUsed(context.Context, int64) (bool, error)  { return false, nil }

type fakeReports struct {
	saved  *domain.Report
	exists bool
}

func (f *fakeReports) List(context.Context) ([]domain.Report, error) { return nil, nil }
func (f *fakeReports) GetByID(context.Context, int64) (*domain.Report, error) {
	return f.saved, nil
}
func (f *fakeReports) Create(_ context.Context, r *domain.Report) error {
	if f.exists {
		return domain.ErrReportExists
	}
	r.ID = 1
	f.saved = r
	return nil
}
func (f *fakeReports) Update(context.Context, *domain.Report) error { return nil }
func (f *fakeReports) Delete(context.Context, int64) error          { return nil }

func newSvc(rep *fakeReports) domain.ReportUsecase {
	users := fakeUsers{byID: map[int64]*domain.User{
		1: {ID: 1, Role: domain.RoleAdmin},
		2: {ID: 2, Role: domain.RoleStaff},
	}}
	aspects := fakeAspects{list: []domain.Aspect{
		{ID: 10, Name: "Kedisiplinan", RoleType: domain.RoleTypeStaff},
		{ID: 11, Name: "Komunikasi", RoleType: domain.RoleTypeStaff},
		{ID: 20, Name: "Kepemimpinan", RoleType: domain.RoleTypeBPH},
	}}
	return NewReportService(rep, users, aspects)
}

func in(uid int64, eval string, s ...domain.ScoreInput) domain.ReportInput {
	return domain.ReportInput{UserID: uid, OverallEvaluation: eval, Scores: s}
}

func TestCreateValidation(t *testing.T) {
	tests := []struct {
		name    string
		in      domain.ReportInput
		wantErr bool
	}{
		{"valid", in(2, "baik", domain.ScoreInput{AspectID: 10, Score: 80}, domain.ScoreInput{AspectID: 11, Score: 100}), false},
		{"batas 0", in(2, "baik", domain.ScoreInput{AspectID: 10, Score: 0}, domain.ScoreInput{AspectID: 11, Score: 0}), false},
		{"di bawah 0", in(2, "baik", domain.ScoreInput{AspectID: 10, Score: -1}, domain.ScoreInput{AspectID: 11, Score: 5}), true},
		{"di atas 100", in(2, "baik", domain.ScoreInput{AspectID: 10, Score: 101}, domain.ScoreInput{AspectID: 11, Score: 5}), true},
		{"evaluasi kosong", in(2, "   ", domain.ScoreInput{AspectID: 10, Score: 80}, domain.ScoreInput{AspectID: 11, Score: 80}), true},
		{"tanpa nilai", in(2, "baik"), true},
		{"aspek kurang", in(2, "baik", domain.ScoreInput{AspectID: 10, Score: 80}), true},
		{"aspek role lain", in(2, "baik", domain.ScoreInput{AspectID: 10, Score: 80}, domain.ScoreInput{AspectID: 11, Score: 80}, domain.ScoreInput{AspectID: 20, Score: 80}), true},
		{"aspek duplikat", in(2, "baik", domain.ScoreInput{AspectID: 10, Score: 80}, domain.ScoreInput{AspectID: 10, Score: 80}), true},
		{"user admin", in(1, "baik", domain.ScoreInput{AspectID: 10, Score: 80}), true},
		{"user tidak ada", in(99, "baik"), true},
		{"user kosong", in(0, "baik"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newSvc(&fakeReports{saved: &domain.Report{}}).Create(context.Background(), 1, tt.in)
			var verr domain.ValidationErrors
			if tt.wantErr && !errors.As(err, &verr) {
				t.Fatalf("want validation error, got %v", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCreateSetsAverageAndCreatedBy(t *testing.T) {
	rep := &fakeReports{}
	_, err := newSvc(rep).Create(context.Background(), 1,
		in(2, " baik ", domain.ScoreInput{AspectID: 10, Score: 85}, domain.ScoreInput{AspectID: 11, Score: 90}))
	if err != nil {
		t.Fatal(err)
	}
	if rep.saved.AverageScore != 87.5 || rep.saved.CreatedBy != 1 || rep.saved.OverallEvaluation != "baik" {
		t.Fatalf("saved = %+v", rep.saved)
	}
}

func TestCreateDuplicateReport(t *testing.T) {
	_, err := newSvc(&fakeReports{exists: true}).Create(context.Background(), 1,
		in(2, "baik", domain.ScoreInput{AspectID: 10, Score: 85}, domain.ScoreInput{AspectID: 11, Score: 90}))
	if !errors.Is(err, domain.ErrReportExists) {
		t.Fatalf("got %v", err)
	}
}

func TestAverage(t *testing.T) {
	if got := Average(85+90+80+88, 4); got != 85.75 {
		t.Fatalf("got %v", got)
	}
	if got := Average(100+100+99, 3); got != 99.67 {
		t.Fatalf("got %v", got)
	}
	if Average(0, 0) != 0 {
		t.Fatal("kosong harus 0")
	}
}
