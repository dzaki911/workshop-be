package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

type migration struct {
	version int64
	name    string
	upSQL   string
	downSQL string
}

func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	byVersion := map[int64]*migration{}
	for _, e := range entries {
		name := e.Name()
		var dir string
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			dir = "up"
		case strings.HasSuffix(name, ".down.sql"):
			dir = "down"
		default:
			continue
		}
		verStr, rest, _ := strings.Cut(name, "_")
		ver, err := strconv.ParseInt(verStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("nama migrasi tidak valid: %s", name)
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		m := byVersion[ver]
		if m == nil {
			m = &migration{version: ver, name: strings.TrimSuffix(strings.TrimSuffix(rest, ".up.sql"), ".down.sql")}
			byVersion[ver] = m
		}
		if dir == "up" {
			m.upSQL = string(body)
		} else {
			m.downSQL = string(body)
		}
	}
	list := make([]migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.upSQL == "" || m.downSQL == "" {
			return nil, fmt.Errorf("migrasi %06d harus punya file up dan down", m.version)
		}
		list = append(list, *m)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })
	return list, nil
}

func ensureTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	return err
}

func applied(ctx context.Context, db *sql.DB) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func run(ctx context.Context, db *sql.DB, query string, version int64, insert bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return err
	}
	if insert {
		_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Up menjalankan semua migrasi yang belum diterapkan. Mengembalikan versi yang dijalankan.
func Up(ctx context.Context, db *sql.DB, fsys fs.FS) ([]int64, error) {
	migs, err := loadMigrations(fsys)
	if err != nil {
		return nil, err
	}
	if err := ensureTable(ctx, db); err != nil {
		return nil, err
	}
	done, err := applied(ctx, db)
	if err != nil {
		return nil, err
	}
	var ran []int64
	for _, m := range migs {
		if done[m.version] {
			continue
		}
		if err := run(ctx, db, m.upSQL, m.version, true); err != nil {
			return ran, fmt.Errorf("migrasi %06d_%s gagal: %w", m.version, m.name, err)
		}
		ran = append(ran, m.version)
	}
	return ran, nil
}

// Down membatalkan `steps` migrasi terakhir (terbaru lebih dulu).
func Down(ctx context.Context, db *sql.DB, fsys fs.FS, steps int) ([]int64, error) {
	migs, err := loadMigrations(fsys)
	if err != nil {
		return nil, err
	}
	if err := ensureTable(ctx, db); err != nil {
		return nil, err
	}
	done, err := applied(ctx, db)
	if err != nil {
		return nil, err
	}
	var ran []int64
	for i := len(migs) - 1; i >= 0 && steps > 0; i-- {
		m := migs[i]
		if !done[m.version] {
			continue
		}
		if err := run(ctx, db, m.downSQL, m.version, false); err != nil {
			return ran, fmt.Errorf("rollback %06d_%s gagal: %w", m.version, m.name, err)
		}
		ran = append(ran, m.version)
		steps--
	}
	return ran, nil
}
