package domain

import (
	"context"
	"time"
)

const (
	MinScore = 0
	MaxScore = 100
)

type ReportScore struct {
	ID         int64  `json:"-"`
	ReportID   int64  `json:"-"`
	AspectID   int64  `json:"aspect_id"`
	AspectName string `json:"aspect_name"`
	Score      int    `json:"score"`
}

// Report adalah raport satu fungsionaris. Field User* diisi lewat join.
type Report struct {
	ID                int64         `json:"id"`
	UserID            int64         `json:"user_id"`
	UserName          string        `json:"user_name"`
	UserRole          Role          `json:"user_role"`
	CreatedBy         int64         `json:"created_by"`
	AverageScore      float64       `json:"average_score"`
	OverallEvaluation string        `json:"overall_evaluation"`
	Scores            []ReportScore `json:"scores,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// ScoreInput adalah nilai satu aspek dari klien.
type ScoreInput struct {
	AspectID int64 `json:"aspect_id"`
	Score    int   `json:"score"`
}

// ReportInput adalah payload create/update raport.
// created_by sengaja tidak ada: identitas admin diambil dari sesi.
// Pada update, user_id diabaikan (raport tidak bisa dipindah ke fungsionaris lain).
type ReportInput struct {
	UserID            int64        `json:"user_id"`
	OverallEvaluation string       `json:"overall_evaluation"`
	Scores            []ScoreInput `json:"scores"`
}

type ReportRepository interface {
	List(ctx context.Context) ([]Report, error)
	// GetByID mengembalikan raport lengkap dengan nilai per aspek.
	GetByID(ctx context.Context, id int64) (*Report, error)
	// Create menyimpan raport + nilai dalam satu transaksi. Mengembalikan ErrReportExists
	// bila fungsionaris sudah punya raport.
	Create(ctx context.Context, r *Report) error
	// Update mengganti evaluasi, rata-rata, dan seluruh nilai dalam satu transaksi.
	Update(ctx context.Context, r *Report) error
	Delete(ctx context.Context, id int64) error
}

type ReportUsecase interface {
	List(ctx context.Context) ([]Report, error)
	Get(ctx context.Context, id int64) (*Report, error)
	Create(ctx context.Context, adminID int64, in ReportInput) (*Report, error)
	Update(ctx context.Context, id int64, in ReportInput) (*Report, error)
	Delete(ctx context.Context, id int64) error
}
