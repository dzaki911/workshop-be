package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"workshop-be/domain"
)

type reportService struct {
	reports domain.ReportRepository
	users   domain.UserRepository
	aspects domain.AspectRepository
}

func NewReportService(reports domain.ReportRepository, users domain.UserRepository, aspects domain.AspectRepository) domain.ReportUsecase {
	return &reportService{reports: reports, users: users, aspects: aspects}
}

func (s *reportService) List(ctx context.Context) ([]domain.Report, error) {
	return s.reports.List(ctx)
}

func (s *reportService) Get(ctx context.Context, id int64) (*domain.Report, error) {
	return s.reports.GetByID(ctx, id)
}

func (s *reportService) Create(ctx context.Context, adminID int64, in domain.ReportInput) (*domain.Report, error) {
	if in.UserID <= 0 {
		return nil, domain.ValidationErrors{{Field: "user_id", Message: "Fungsionaris wajib dipilih"}}
	}
	target, err := s.users.GetByID(ctx, in.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ValidationErrors{{Field: "user_id", Message: "Fungsionaris tidak ditemukan"}}
	}
	if err != nil {
		return nil, err
	}

	rep, err := s.build(ctx, target, in)
	if err != nil {
		return nil, err
	}
	rep.CreatedBy = adminID
	if err := s.reports.Create(ctx, rep); err != nil {
		return nil, err
	}
	return s.reports.GetByID(ctx, rep.ID)
}

func (s *reportService) Update(ctx context.Context, id int64, in domain.ReportInput) (*domain.Report, error) {
	cur, err := s.reports.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	target, err := s.users.GetByID(ctx, cur.UserID)
	if err != nil {
		return nil, err
	}
	rep, err := s.build(ctx, target, in)
	if err != nil {
		return nil, err
	}
	rep.ID = id
	if err := s.reports.Update(ctx, rep); err != nil {
		return nil, err
	}
	return s.reports.GetByID(ctx, id)
}

func (s *reportService) Delete(ctx context.Context, id int64) error {
	return s.reports.Delete(ctx, id)
}

// build memvalidasi input terhadap fungsionaris target dan aspek yang berlaku,
// lalu menghitung rata-rata (jumlah nilai / jumlah aspek, tanpa bobot).
func (s *reportService) build(ctx context.Context, target *domain.User, in domain.ReportInput) (*domain.Report, error) {
	var errs domain.ValidationErrors

	roleType, ok := target.RoleType()
	if !ok {
		return nil, domain.ValidationErrors{{Field: "user_id", Message: "User yang dipilih bukan fungsionaris (bph/staff)"}}
	}
	evaluation := strings.TrimSpace(in.OverallEvaluation)
	if evaluation == "" {
		errs.Add("overall_evaluation", "Evaluasi menyeluruh wajib diisi")
	}

	applicable, err := s.aspects.List(ctx, &roleType)
	if err != nil {
		return nil, err
	}
	if len(applicable) == 0 {
		errs.Add("scores", fmt.Sprintf("Belum ada aspek penilaian untuk role %s", roleType))
	}
	byID := make(map[int64]domain.Aspect, len(applicable))
	for _, a := range applicable {
		byID[a.ID] = a
	}

	if len(in.Scores) == 0 {
		errs.Add("scores", "Raport tanpa nilai aspek tidak dapat disimpan")
	}

	seen := map[int64]bool{}
	scores := make([]domain.ReportScore, 0, len(in.Scores))
	sum := 0
	for i, sc := range in.Scores {
		a, known := byID[sc.AspectID]
		if !known {
			errs.Add(fmt.Sprintf("scores[%d].aspect_id", i), fmt.Sprintf("Aspek %d tidak sesuai dengan role %s", sc.AspectID, target.Role))
			continue
		}
		if seen[sc.AspectID] {
			errs.Add(fmt.Sprintf("scores[%d].aspect_id", i), fmt.Sprintf("Aspek %d diisi lebih dari sekali", sc.AspectID))
			continue
		}
		seen[sc.AspectID] = true
		if sc.Score < domain.MinScore || sc.Score > domain.MaxScore {
			errs.Add(fmt.Sprintf("scores[%d].score", i), fmt.Sprintf("Nilai harus %d–%d", domain.MinScore, domain.MaxScore))
			continue
		}
		sum += sc.Score
		scores = append(scores, domain.ReportScore{AspectID: a.ID, AspectName: a.Name, Score: sc.Score})
	}
	for _, a := range applicable {
		if len(in.Scores) > 0 && !seen[a.ID] {
			errs.Add("scores", fmt.Sprintf("Aspek %q (id %d) wajib memiliki nilai", a.Name, a.ID))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}

	return &domain.Report{
		UserID:            target.ID,
		AverageScore:      Average(sum, len(applicable)),
		OverallEvaluation: evaluation,
		Scores:            scores,
	}, nil
}

// Average = jumlah / banyak aspek, dibulatkan 2 desimal.
func Average(sum, count int) float64 {
	if count == 0 {
		return 0
	}
	return math.Round(float64(sum)/float64(count)*100) / 100
}
