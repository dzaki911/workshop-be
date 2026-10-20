// Seed lokal: akun uji (admin, BPH, staff) dan aspek contoh. Idempotent.
// HANYA untuk lingkungan lokal; jangan dijalankan di production.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"

	"workshop-be/config"
	"workshop-be/infra/database"
)

type seedUser struct{ name, email, role, position, ministry string }

var users = []seedUser{
	{"Admin Uji", "admin@raport.local", "admin", "", ""},
	{"Budi Ketua", "bph@raport.local", "bph", "Ketua Umum", ""},
	{"Sari Sekretaris", "bph2@raport.local", "bph", "Sekretaris", ""},
	{"Andi Staff", "staff@raport.local", "staff", "Staff", "Kementerian Pendidikan"},
	{"Dewi Staff", "staff2@raport.local", "staff", "Staff", "Kementerian Humas"},
}

var aspects = []struct{ name, desc, roleType string }{
	{"Kepemimpinan", "Kemampuan memimpin dan mengarahkan tim", "bph"},
	{"Pengambilan keputusan", "Ketepatan dan kecepatan mengambil keputusan", "bph"},
	{"Koordinasi tim", "Kemampuan mengoordinasi anggota dan divisi", "bph"},
	{"Manajemen program kerja", "Perencanaan dan pelaksanaan program kerja", "bph"},
	{"Problem solving", "Kemampuan menyelesaikan masalah", "bph"},
	{"Kedisiplinan", "Ketepatan waktu dan kepatuhan aturan", "staff"},
	{"Tanggung jawab", "Menyelesaikan tugas sesuai amanah", "staff"},
	{"Keaktifan", "Kehadiran dan partisipasi", "staff"},
	{"Komunikasi", "Kejelasan dan responsivitas komunikasi", "staff"},
	{"Kolaborasi tim", "Kerja sama dengan anggota lain", "staff"},
}

func main() {
	cfg := config.Load()
	if cfg.AppEnv != "development" {
		log.Fatal("seed hanya boleh dijalankan saat APP_ENV=development")
	}
	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		password = "Password123!"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DB.URL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for _, u := range users {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (name, email, password, role, position, ministry)
			 VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''))
			 ON CONFLICT (email) DO NOTHING`,
			u.name, u.email, string(hash), u.role, u.position, u.ministry)
		if err != nil {
			log.Fatalf("seed user %s: %v", u.email, err)
		}
	}
	for _, a := range aspects {
		_, err := db.ExecContext(ctx,
			`INSERT INTO assessment_aspects (name, description, role_type)
			 SELECT $1::varchar, $2::text, $3::aspect_role_type
			 WHERE NOT EXISTS (SELECT 1 FROM assessment_aspects WHERE name = $1::varchar AND role_type = $3::aspect_role_type)`,
			a.name, a.desc, a.roleType)
		if err != nil {
			log.Fatalf("seed aspek %s: %v", a.name, err)
		}
	}
	fmt.Printf("seed selesai: %d user, %d aspek (password akun uji: nilai SEED_PASSWORD atau default dokumentasi)\n", len(users), len(aspects))
}
