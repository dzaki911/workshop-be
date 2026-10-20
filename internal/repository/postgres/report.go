package postgres

import (
	"context"
	"database/sql"
	"errors"

	"workshop-be/domain"
)

type ReportRepository struct{ db *sql.DB }

func NewReportRepository(db *sql.DB) *ReportRepository { return &ReportRepository{db: db} }

const reportSelect = `
	SELECT r.id, r.user_id, u.name, u.role, r.created_by, r.average_score::float8,
	       r.overall_evaluation, r.created_at, r.updated_at
	FROM reports r JOIN users u ON u.id = r.user_id`

func scanReport(s interface{ Scan(...any) error }) (*domain.Report, error) {
	var r domain.Report
	if err := s.Scan(&r.ID, &r.UserID, &r.UserName, &r.UserRole, &r.CreatedBy, &r.AverageScore,
		&r.OverallEvaluation, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *ReportRepository) List(ctx context.Context) ([]domain.Report, error) {
	rows, err := r.db.QueryContext(ctx, reportSelect+` ORDER BY r.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Report{}
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rep)
	}
	return out, rows.Err()
}

func (r *ReportRepository) GetByID(ctx context.Context, id int64) (*domain.Report, error) {
	rep, err := scanReport(r.db.QueryRowContext(ctx, reportSelect+` WHERE r.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT s.id, s.report_id, s.aspect_id, a.name, s.score
		 FROM report_scores s JOIN assessment_aspects a ON a.id = s.aspect_id
		 WHERE s.report_id = $1 ORDER BY s.aspect_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rep.Scores = []domain.ReportScore{}
	for rows.Next() {
		var s domain.ReportScore
		if err := rows.Scan(&s.ID, &s.ReportID, &s.AspectID, &s.AspectName, &s.Score); err != nil {
			return nil, err
		}
		rep.Scores = append(rep.Scores, s)
	}
	return rep, rows.Err()
}

func (r *ReportRepository) Create(ctx context.Context, rep *domain.Report) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO reports (user_id, created_by, average_score, overall_evaluation)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`,
		rep.UserID, rep.CreatedBy, rep.AverageScore, rep.OverallEvaluation).
		Scan(&rep.ID, &rep.CreatedAt, &rep.UpdatedAt)
	if code, _ := pgCode(err); code == codeUniqueViolation {
		return domain.ErrReportExists
	}
	if err != nil {
		return err
	}
	if err := insertScores(ctx, tx, rep.ID, rep.Scores); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ReportRepository) Update(ctx context.Context, rep *domain.Report) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`UPDATE reports SET average_score = $1, overall_evaluation = $2, updated_at = now()
		 WHERE id = $3 RETURNING updated_at`,
		rep.AverageScore, rep.OverallEvaluation, rep.ID).Scan(&rep.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM report_scores WHERE report_id = $1`, rep.ID); err != nil {
		return err
	}
	if err := insertScores(ctx, tx, rep.ID, rep.Scores); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ReportRepository) Delete(ctx context.Context, id int64) error {
	// report_scores terhapus otomatis lewat ON DELETE CASCADE.
	res, err := r.db.ExecContext(ctx, `DELETE FROM reports WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func insertScores(ctx context.Context, tx *sql.Tx, reportID int64, scores []domain.ReportScore) error {
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO report_scores (report_id, aspect_id, score) VALUES ($1, $2, $3)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, s := range scores {
		if _, err := stmt.ExecContext(ctx, reportID, s.AspectID, s.Score); err != nil {
			return err
		}
	}
	return nil
}
