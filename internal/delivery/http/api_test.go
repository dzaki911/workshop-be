package http_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"workshop-be/domain"
	"workshop-be/infra/database"
	"workshop-be/infra/database/migrations"
	delivery "workshop-be/internal/delivery/http"
	"workshop-be/internal/repository/memory"
	pgrepo "workshop-be/internal/repository/postgres"
	"workshop-be/internal/service"
)

// Suite ini dijalankan terhadap dua backend dengan assertion yang sama:
//   - "memory"   : repository in-memory, selalu jalan (tanpa database)
//   - "postgres" : PostgreSQL asli, hanya jika TEST_DATABASE_URL di-set
//     (database akan di-reset: migrate down lalu up, jangan arahkan ke database penting!)
//
// Seed identik di keduanya, sehingga ID deterministik:
//
//	users  : 1 admin, 2 Budi Ketua (bph), 3 Sari Sekretaris (bph), 4 Andi Staff, 5 Dewi Staff
//	aspek  : 1-5 bph, 6-10 staff

var (
	userSeed = []struct {
		name, email string
		role        domain.Role
	}{
		{"Admin Uji", "admin@raport.local", domain.RoleAdmin},
		{"Budi Ketua", "bph@raport.local", domain.RoleBPH},
		{"Sari Sekretaris", "bph2@raport.local", domain.RoleBPH},
		{"Andi Staff", "staff@raport.local", domain.RoleStaff},
		{"Dewi Staff", "staff2@raport.local", domain.RoleStaff},
	}
	aspectSeed = []struct {
		name string
		rt   domain.RoleType
	}{
		{"Kepemimpinan", domain.RoleTypeBPH}, {"Pengambilan keputusan", domain.RoleTypeBPH},
		{"Koordinasi tim", domain.RoleTypeBPH}, {"Manajemen program kerja", domain.RoleTypeBPH},
		{"Problem solving", domain.RoleTypeBPH},
		{"Kedisiplinan", domain.RoleTypeStaff}, {"Tanggung jawab", domain.RoleTypeStaff},
		{"Keaktifan", domain.RoleTypeStaff}, {"Komunikasi", domain.RoleTypeStaff},
		{"Kolaborasi tim", domain.RoleTypeStaff},
	}
)

type repos struct {
	users   domain.UserRepository
	aspects domain.AspectRepository
	reports domain.ReportRepository
}

type builder func(t *testing.T) repos

func memoryBuilder(t *testing.T) repos {
	s := memory.NewStore()
	for _, u := range userSeed {
		s.AddUser(u.name, u.email, u.role)
	}
	for _, a := range aspectSeed {
		s.AddAspect(a.name, a.rt)
	}
	return repos{s.Users(), s.Aspects(), s.Reports()}
}

func postgresBuilder(url string) builder {
	return func(t *testing.T) repos {
		ctx := context.Background()
		db, err := database.Open(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		if _, err := database.Down(ctx, db, migrations.FS, 100); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Up(ctx, db, migrations.FS); err != nil {
			t.Fatal(err)
		}
		seedPostgres(t, db)
		return repos{pgrepo.NewUserRepository(db), pgrepo.NewAspectRepository(db), pgrepo.NewReportRepository(db)}
	}
}

func seedPostgres(t *testing.T, db *sql.DB) {
	for _, u := range userSeed {
		if _, err := db.Exec(`INSERT INTO users (name, email, password, role) VALUES ($1,$2,'x',$3)`, u.name, u.email, string(u.role)); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range aspectSeed {
		if _, err := db.Exec(`INSERT INTO assessment_aspects (name, role_type) VALUES ($1,$2)`, a.name, string(a.rt)); err != nil {
			t.Fatal(err)
		}
	}
}

func newHandler(r repos, adminEnabled bool) http.Handler {
	h := delivery.NewHandler(
		service.NewUserService(r.users),
		service.NewAspectService(r.aspects),
		service.NewReportService(r.reports, r.users, r.aspects),
	)
	return delivery.NewRouter(h, delivery.DevAdmin(adminEnabled, "admin@raport.local", r.users))
}

// ---- helpers ----

type client struct {
	t *testing.T
	h http.Handler
}

type result struct {
	code int
	body map[string]any
}

// do mengirim request. body: nil, string (dikirim apa adanya), atau value yang di-marshal ke JSON.
func (c client) do(method, path string, body any) result {
	c.t.Helper()
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)

	res := result{code: rec.Code, body: map[string]any{}}
	if err := json.Unmarshal(rec.Body.Bytes(), &res.body); err != nil {
		c.t.Fatalf("%s %s: response bukan JSON: %q", method, path, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		c.t.Fatalf("%s %s: Content-Type = %q", method, path, ct)
	}
	return res
}

func (r result) status(t *testing.T, want int) result {
	t.Helper()
	if r.code != want {
		t.Fatalf("status = %d, want %d; body = %v", r.code, want, r.body)
	}
	// format seragam: status success/error sesuai kode
	wantStatus := "success"
	if want >= 400 {
		wantStatus = "error"
	}
	if r.body["status"] != wantStatus {
		t.Fatalf("body.status = %v, want %s", r.body["status"], wantStatus)
	}
	if m, _ := r.body["message"].(string); m == "" {
		t.Fatalf("message kosong: %v", r.body)
	}
	return r
}

func (r result) data() map[string]any {
	d, _ := r.body["data"].(map[string]any)
	return d
}

func (r result) list() []any {
	d, _ := r.body["data"].([]any)
	return d
}

func (r result) errorFields() []string {
	var out []string
	es, _ := r.body["errors"].([]any)
	for _, e := range es {
		m := e.(map[string]any)
		if m["message"] == "" || m["message"] == nil {
			panic("errors[].message kosong")
		}
		out = append(out, m["field"].(string))
	}
	return out
}

func (r result) hasErrorField(t *testing.T, field string) {
	t.Helper()
	for _, f := range r.errorFields() {
		if f == field {
			return
		}
	}
	t.Fatalf("errors tidak memuat field %q: %v", field, r.body["errors"])
}

func num(t *testing.T, v any) float64 {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("bukan angka: %#v", v)
	}
	return f
}

type sc struct {
	AspectID int `json:"aspect_id"`
	Score    int `json:"score"`
}

func reportBody(userID int, eval string, scores ...sc) map[string]any {
	return map[string]any{"user_id": userID, "overall_evaluation": eval, "scores": scores}
}

// staffScores: aspek 6-10, rata-rata (85+90+80+88+88)/5 = 86.2
func staffScores() []sc { return []sc{{6, 85}, {7, 90}, {8, 80}, {9, 88}, {10, 88}} }

// bphScores: aspek 1-5, rata-rata (70+80+90+100+60)/5 = 80
func bphScores() []sc { return []sc{{1, 70}, {2, 80}, {3, 90}, {4, 100}, {5, 60}} }

const (
	adm  = "/api/v1/admin"
	aspP = adm + "/aspects"
	repP = adm + "/reports"
)

// ---- entry point ----

func TestAPI(t *testing.T) {
	backends := map[string]builder{"memory": memoryBuilder}
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		backends["postgres"] = postgresBuilder(url)
	}
	for name, b := range backends {
		b := b
		t.Run(name, func(t *testing.T) {
			for _, tc := range cases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					tc.fn(t, client{t: t, h: newHandler(b(t), true)})
				})
			}
			t.Run("admin ditolak saat bukan development", func(t *testing.T) {
				c := client{t: t, h: newHandler(b(t), false)}
				c.do("GET", "/health", nil).status(t, 200)
				for _, p := range []string{adm + "/users", aspP, repP, aspP + "/6", repP + "/1"} {
					c.do("GET", p, nil).status(t, 401)
				}
				c.do("POST", repP, reportBody(4, "x", staffScores()...)).status(t, 401)
				c.do("DELETE", aspP+"/6", nil).status(t, 401)
			})
		})
	}
}

type testCase struct {
	name string
	fn   func(t *testing.T, c client)
}

var cases = []testCase{
	// ===== umum =====
	{"health", func(t *testing.T, c client) {
		r := c.do("GET", "/health", nil).status(t, 200)
		if r.data()["status"] != "ok" {
			t.Fatalf("data = %v", r.data())
		}
	}},
	{"endpoint auth belum diimplementasikan (501)", func(t *testing.T, c client) {
		c.do("POST", "/api/v1/auth/login", `{}`).status(t, 501)
		c.do("POST", "/api/v1/auth/logout", nil).status(t, 501)
		c.do("GET", "/api/v1/me", nil).status(t, 501)
		c.do("GET", "/api/v1/me/report", nil).status(t, 501)
	}},
	{"route tidak dikenal 404 berformat JSON", func(t *testing.T, c client) {
		c.do("GET", "/api/v1/nope", nil).status(t, 404)
	}},

	// ===== users =====
	{"users: daftar semua fungsionaris tanpa admin dan tanpa password", func(t *testing.T, c client) {
		r := c.do("GET", adm+"/users", nil).status(t, 200)
		if len(r.list()) != 4 {
			t.Fatalf("len = %d, want 4", len(r.list()))
		}
		for _, u := range r.list() {
			m := u.(map[string]any)
			if m["role"] == "admin" {
				t.Fatal("admin tidak boleh muncul")
			}
			for _, banned := range []string{"password", "password_hash", "PasswordHash"} {
				if _, ok := m[banned]; ok {
					t.Fatalf("response membocorkan %s", banned)
				}
			}
		}
	}},
	{"users: filter role", func(t *testing.T, c client) {
		for role, want := range map[string]int{"bph": 2, "staff": 2} {
			r := c.do("GET", adm+"/users?role="+role, nil).status(t, 200)
			if len(r.list()) != want {
				t.Fatalf("role=%s len=%d want %d", role, len(r.list()), want)
			}
			for _, u := range r.list() {
				if u.(map[string]any)["role"] != role {
					t.Fatalf("role bocor: %v", u)
				}
			}
		}
	}},
	{"users: role tidak valid 400", func(t *testing.T, c client) {
		c.do("GET", adm+"/users?role=admin", nil).status(t, 400).hasErrorField(t, "role")
		c.do("GET", adm+"/users?role=xyz", nil).status(t, 400)
	}},

	// ===== aspects =====
	{"aspects: daftar dan filter role_type", func(t *testing.T, c client) {
		if n := len(c.do("GET", aspP, nil).status(t, 200).list()); n != 10 {
			t.Fatalf("semua = %d", n)
		}
		for rt, want := range map[string]int{"bph": 5, "staff": 5} {
			r := c.do("GET", aspP+"?role_type="+rt, nil).status(t, 200)
			if len(r.list()) != want {
				t.Fatalf("%s = %d", rt, len(r.list()))
			}
			for _, a := range r.list() {
				if a.(map[string]any)["role_type"] != rt {
					t.Fatalf("role_type bocor: %v", a)
				}
			}
		}
		c.do("GET", aspP+"?role_type=admin", nil).status(t, 400).hasErrorField(t, "role_type")
	}},
	{"aspects: detail", func(t *testing.T, c client) {
		d := c.do("GET", aspP+"/6", nil).status(t, 200).data()
		if d["name"] != "Kedisiplinan" || d["role_type"] != "staff" || num(t, d["id"]) != 6 {
			t.Fatalf("data = %v", d)
		}
		c.do("GET", aspP+"/999", nil).status(t, 404)
		for _, bad := range []string{"abc", "0", "-1"} {
			c.do("GET", aspP+"/"+bad, nil).status(t, 400).hasErrorField(t, "id")
		}
	}},
	{"aspects: tambah sukses 201 dan muncul di daftar", func(t *testing.T, c client) {
		r := c.do("POST", aspP, map[string]any{"name": "  Inisiatif ", "description": "Proaktif", "role_type": "STAFF"}).status(t, 201)
		d := r.data()
		if d["name"] != "Inisiatif" || d["role_type"] != "staff" || d["description"] != "Proaktif" || num(t, d["id"]) != 11 {
			t.Fatalf("data = %v", d)
		}
		got := c.do("GET", aspP+"?role_type=staff", nil).status(t, 200).list()
		if len(got) != 6 {
			t.Fatalf("staff aspek = %d, want 6", len(got))
		}
	}},
	{"aspects: nama sama boleh di role_type berbeda, duplikat di role sama 409", func(t *testing.T, c client) {
		c.do("POST", aspP, map[string]any{"name": "Komunikasi", "role_type": "bph"}).status(t, 201)
		c.do("POST", aspP, map[string]any{"name": "Komunikasi", "role_type": "bph"}).status(t, 409)
		c.do("POST", aspP, map[string]any{"name": "Komunikasi", "role_type": "staff"}).status(t, 409) // sudah ada di seed
	}},
	{"aspects: validasi tambah 400", func(t *testing.T, c client) {
		c.do("POST", aspP, map[string]any{"name": "", "role_type": "bph"}).status(t, 400).hasErrorField(t, "name")
		c.do("POST", aspP, map[string]any{"name": "   ", "role_type": "bph"}).status(t, 400).hasErrorField(t, "name")
		c.do("POST", aspP, map[string]any{"name": strings.Repeat("a", 101), "role_type": "bph"}).status(t, 400).hasErrorField(t, "name")
		c.do("POST", aspP, map[string]any{"name": "X", "role_type": "admin"}).status(t, 400).hasErrorField(t, "role_type")
		c.do("POST", aspP, map[string]any{"name": "X"}).status(t, 400).hasErrorField(t, "role_type")
		r := c.do("POST", aspP, map[string]any{}).status(t, 400)
		r.hasErrorField(t, "name")
		r.hasErrorField(t, "role_type")
		c.do("POST", aspP, `{bukan json`).status(t, 400).hasErrorField(t, "body")
		c.do("POST", aspP, nil).status(t, 400).hasErrorField(t, "body")
		c.do("POST", aspP, `{"name": 123, "role_type": "bph"}`).status(t, 400)
		if n := len(c.do("GET", aspP, nil).list()); n != 10 {
			t.Fatalf("aspek bertambah padahal validasi gagal: %d", n)
		}
	}},
	{"aspects: ubah", func(t *testing.T, c client) {
		d := c.do("PUT", aspP+"/6", map[string]any{"name": "Disiplin", "description": "baru", "role_type": "staff"}).status(t, 200).data()
		if d["name"] != "Disiplin" || d["description"] != "baru" {
			t.Fatalf("data = %v", d)
		}
		if g := c.do("GET", aspP+"/6", nil).data(); g["name"] != "Disiplin" {
			t.Fatalf("tidak tersimpan: %v", g)
		}
		c.do("PUT", aspP+"/999", map[string]any{"name": "X", "role_type": "bph"}).status(t, 404)
		c.do("PUT", aspP+"/abc", map[string]any{"name": "X", "role_type": "bph"}).status(t, 400).hasErrorField(t, "id")
		c.do("PUT", aspP+"/6", map[string]any{"name": "", "role_type": "staff"}).status(t, 400).hasErrorField(t, "name")
		c.do("PUT", aspP+"/6", `rusak`).status(t, 400)
		c.do("PUT", aspP+"/6", map[string]any{"name": "Tanggung jawab", "role_type": "staff"}).status(t, 409) // bentrok dengan aspek 7
	}},
	{"aspects: role_type aspek yang belum dipakai boleh diubah", func(t *testing.T, c client) {
		d := c.do("PUT", aspP+"/6", map[string]any{"name": "Kedisiplinan", "role_type": "bph"}).status(t, 200).data()
		if d["role_type"] != "bph" {
			t.Fatalf("data = %v", d)
		}
	}},
	{"aspects: hapus yang belum dipakai", func(t *testing.T, c client) {
		c.do("DELETE", aspP+"/6", nil).status(t, 200)
		c.do("GET", aspP+"/6", nil).status(t, 404)
		c.do("DELETE", aspP+"/6", nil).status(t, 404)
		c.do("DELETE", aspP+"/abc", nil).status(t, 400)
	}},
	{"aspects: yang sudah dipakai raport tidak bisa dihapus / diubah role-nya, nama tetap bisa diubah", func(t *testing.T, c client) {
		c.do("POST", repP, reportBody(4, "Baik", staffScores()...)).status(t, 201)
		c.do("DELETE", aspP+"/6", nil).status(t, 409)
		c.do("PUT", aspP+"/6", map[string]any{"name": "Kedisiplinan", "role_type": "bph"}).status(t, 409)
		c.do("PUT", aspP+"/6", map[string]any{"name": "Disiplin baru", "role_type": "staff"}).status(t, 200)
		c.do("GET", aspP+"/6", nil).status(t, 200)
		// aspek yang tidak dipakai (BPH) tetap bisa dihapus
		c.do("DELETE", aspP+"/1", nil).status(t, 200)
	}},
	{"aspects: setelah raport dihapus aspek bisa dihapus", func(t *testing.T, c client) {
		id := num(t, c.do("POST", repP, reportBody(4, "Baik", staffScores()...)).status(t, 201).data()["id"])
		c.do("DELETE", aspP+"/6", nil).status(t, 409)
		c.do("DELETE", fmt.Sprintf("%s/%d?confirm=true", repP, int(id)), nil).status(t, 200)
		c.do("DELETE", aspP+"/6", nil).status(t, 200)
	}},

	// ===== reports: create =====
	{"reports: buat raport staff 201 (rata-rata, nilai, created_by dari sesi)", func(t *testing.T, c client) {
		body := reportBody(4, "  Performa baik.  ", staffScores()...)
		body["created_by"] = 3 // harus diabaikan
		d := c.do("POST", repP, body).status(t, 201).data()
		if num(t, d["average_score"]) != 86.2 {
			t.Fatalf("average_score = %v", d["average_score"])
		}
		if num(t, d["created_by"]) != 1 {
			t.Fatalf("created_by = %v, harus admin sesi (1)", d["created_by"])
		}
		if num(t, d["user_id"]) != 4 || d["user_name"] != "Andi Staff" || d["user_role"] != "staff" {
			t.Fatalf("identitas = %v", d)
		}
		if d["overall_evaluation"] != "Performa baik." {
			t.Fatalf("evaluasi tidak di-trim: %q", d["overall_evaluation"])
		}
		scores := d["scores"].([]any)
		if len(scores) != 5 {
			t.Fatalf("scores = %d", len(scores))
		}
		first := scores[0].(map[string]any)
		if num(t, first["aspect_id"]) != 6 || first["aspect_name"] != "Kedisiplinan" || num(t, first["score"]) != 85 {
			t.Fatalf("score[0] = %v", first)
		}
	}},
	{"reports: buat raport BPH dengan aspek BPH", func(t *testing.T, c client) {
		d := c.do("POST", repP, reportBody(2, "Kepemimpinan baik", bphScores()...)).status(t, 201).data()
		if num(t, d["average_score"]) != 80 || d["user_role"] != "bph" {
			t.Fatalf("data = %v", d)
		}
	}},
	{"reports: nilai batas 0 dan 100 diterima", func(t *testing.T, c client) {
		d := c.do("POST", repP, reportBody(4, "ok", sc{6, 0}, sc{7, 100}, sc{8, 0}, sc{9, 100}, sc{10, 0})).status(t, 201).data()
		if num(t, d["average_score"]) != 40 {
			t.Fatalf("average = %v", d["average_score"])
		}
	}},
	{"reports: satu raport per fungsionaris (409), fungsionaris lain tetap bisa", func(t *testing.T, c client) {
		c.do("POST", repP, reportBody(4, "Pertama", staffScores()...)).status(t, 201)
		c.do("POST", repP, reportBody(4, "Kedua", staffScores()...)).status(t, 409)
		c.do("POST", repP, reportBody(5, "Lain", staffScores()...)).status(t, 201)
		if n := len(c.do("GET", repP, nil).list()); n != 2 {
			t.Fatalf("jumlah raport = %d", n)
		}
	}},
	{"reports: validasi create 400 dan tidak ada yang tersimpan", func(t *testing.T, c client) {
		with := func(i, score int) []sc {
			s := staffScores()
			s[i].Score = score
			return s
		}
		bad := []struct {
			name  string
			body  any
			field string
		}{
			{"nilai -1", reportBody(4, "ok", with(0, -1)...), "scores[0].score"},
			{"nilai 101", reportBody(4, "ok", with(2, 101)...), "scores[2].score"},
			{"evaluasi kosong", reportBody(4, "", staffScores()...), "overall_evaluation"},
			{"evaluasi spasi", reportBody(4, "   \n ", staffScores()...), "overall_evaluation"},
			{"tanpa nilai", reportBody(4, "ok"), "scores"},
			{"aspek kurang", reportBody(4, "ok", staffScores()[:4]...), "scores"},
			{"aspek role lain", reportBody(4, "ok", sc{6, 80}, sc{7, 80}, sc{8, 80}, sc{9, 80}, sc{1, 80}), "scores[4].aspect_id"},
			{"aspek tidak ada", reportBody(4, "ok", sc{6, 80}, sc{7, 80}, sc{8, 80}, sc{9, 80}, sc{999, 80}), "scores[4].aspect_id"},
			{"aspek duplikat", reportBody(4, "ok", sc{6, 80}, sc{6, 80}, sc{7, 80}, sc{8, 80}, sc{9, 80}), "scores[1].aspect_id"},
			{"user admin", reportBody(1, "ok", staffScores()...), "user_id"},
			{"user tidak ada", reportBody(99, "ok", staffScores()...), "user_id"},
			{"user_id 0", reportBody(0, "ok", staffScores()...), "user_id"},
			{"user_id negatif", reportBody(-3, "ok", staffScores()...), "user_id"},
			{"user_id string", `{"user_id":"abc","overall_evaluation":"x","scores":[]}`, "user_id"},
			{"nilai desimal", `{"user_id":4,"overall_evaluation":"x","scores":[{"aspect_id":6,"score":85.5}]}`, ""},
			{"nilai string", `{"user_id":4,"overall_evaluation":"x","scores":[{"aspect_id":6,"score":"tinggi"}]}`, ""},
			{"json rusak", `{"user_id":`, "body"},
			{"body kosong", nil, "body"},
			{"aspek BPH untuk user staff semua", reportBody(4, "ok", bphScores()...), "scores"},
			{"aspek staff untuk user BPH", reportBody(2, "ok", staffScores()...), "scores"},
		}
		for _, b := range bad {
			r := c.do("POST", repP, b.body).status(t, 400)
			if b.field != "" {
				func() {
					defer func() {
						if recover() != nil {
							t.Fatalf("%s: errors tidak valid: %v", b.name, r.body)
						}
					}()
					r.hasErrorField(t, b.field)
				}()
			}
			if len(r.errorFields()) == 0 {
				t.Fatalf("%s: errors kosong: %v", b.name, r.body)
			}
		}
		if n := len(c.do("GET", repP, nil).status(t, 200).list()); n != 0 {
			t.Fatalf("ada %d raport tersimpan padahal semua request gagal", n)
		}
	}},
	{"reports: error validasi memuat beberapa field sekaligus", func(t *testing.T, c client) {
		r := c.do("POST", repP, reportBody(4, "", sc{6, 150}, sc{1, 5})).status(t, 400)
		r.hasErrorField(t, "overall_evaluation")
		r.hasErrorField(t, "scores[0].score")
		r.hasErrorField(t, "scores[1].aspect_id")
		r.hasErrorField(t, "scores")
	}},

	// ===== reports: read =====
	{"reports: daftar (terbaru dulu, tanpa nilai) dan detail", func(t *testing.T, c client) {
		if n := len(c.do("GET", repP, nil).status(t, 200).list()); n != 0 {
			t.Fatalf("awal harus kosong, got %d", n)
		}
		a := num(t, c.do("POST", repP, reportBody(4, "A", staffScores()...)).status(t, 201).data()["id"])
		b := num(t, c.do("POST", repP, reportBody(2, "B", bphScores()...)).status(t, 201).data()["id"])
		list := c.do("GET", repP, nil).status(t, 200).list()
		if len(list) != 2 || num(t, list[0].(map[string]any)["id"]) != b || num(t, list[1].(map[string]any)["id"]) != a {
			t.Fatalf("urutan/isi daftar salah: %v", list)
		}
		row := list[0].(map[string]any)
		if row["user_name"] != "Budi Ketua" || row["user_role"] != "bph" || num(t, row["average_score"]) != 80 {
			t.Fatalf("row = %v", row)
		}
		if _, has := row["scores"]; has {
			t.Fatal("daftar tidak boleh memuat scores")
		}

		d := c.do("GET", fmt.Sprintf("%s/%d", repP, int(a)), nil).status(t, 200).data()
		if num(t, d["average_score"]) != 86.2 || len(d["scores"].([]any)) != 5 || d["overall_evaluation"] != "A" {
			t.Fatalf("detail = %v", d)
		}
	}},
	{"reports: detail 404 / id tidak valid", func(t *testing.T, c client) {
		c.do("GET", repP+"/999", nil).status(t, 404)
		c.do("GET", repP+"/abc", nil).status(t, 400).hasErrorField(t, "id")
		c.do("GET", repP+"/0", nil).status(t, 400)
	}},

	// ===== reports: update =====
	{"reports: ubah mengganti nilai, evaluasi, dan menghitung ulang rata-rata", func(t *testing.T, c client) {
		id := int(num(t, c.do("POST", repP, reportBody(4, "Awal", staffScores()...)).status(t, 201).data()["id"]))
		path := fmt.Sprintf("%s/%d", repP, id)
		// user_id dan created_by di body harus diabaikan
		body := reportBody(5, "Revisi", sc{6, 100}, sc{7, 100}, sc{8, 100}, sc{9, 100}, sc{10, 99})
		body["created_by"] = 3
		d := c.do("PUT", path, body).status(t, 200).data()
		if num(t, d["average_score"]) != 99.8 || d["overall_evaluation"] != "Revisi" {
			t.Fatalf("data = %v", d)
		}
		if num(t, d["user_id"]) != 4 || num(t, d["created_by"]) != 1 {
			t.Fatalf("user_id/created_by berubah: %v", d)
		}
		g := c.do("GET", path, nil).status(t, 200).data()
		scores := g["scores"].([]any)
		if len(scores) != 5 || num(t, scores[4].(map[string]any)["score"]) != 99 {
			t.Fatalf("nilai tidak diganti: %v", scores)
		}
	}},
	{"reports: ubah dengan input tidak valid 400 dan data lama utuh", func(t *testing.T, c client) {
		id := int(num(t, c.do("POST", repP, reportBody(4, "Awal", staffScores()...)).status(t, 201).data()["id"]))
		path := fmt.Sprintf("%s/%d", repP, id)
		c.do("PUT", path, reportBody(4, "", staffScores()...)).status(t, 400).hasErrorField(t, "overall_evaluation")
		c.do("PUT", path, reportBody(4, "x", sc{6, 101}, sc{7, 1}, sc{8, 1}, sc{9, 1}, sc{10, 1})).status(t, 400).hasErrorField(t, "scores[0].score")
		c.do("PUT", path, reportBody(4, "x", staffScores()[:2]...)).status(t, 400).hasErrorField(t, "scores")
		c.do("PUT", path, reportBody(4, "x", bphScores()...)).status(t, 400)
		c.do("PUT", path, `rusak`).status(t, 400)
		g := c.do("GET", path, nil).status(t, 200).data()
		if g["overall_evaluation"] != "Awal" || num(t, g["average_score"]) != 86.2 || len(g["scores"].([]any)) != 5 {
			t.Fatalf("data lama berubah: %v", g)
		}
	}},
	{"reports: ubah 404 / id tidak valid", func(t *testing.T, c client) {
		c.do("PUT", repP+"/999", reportBody(4, "x", staffScores()...)).status(t, 404)
		c.do("PUT", repP+"/abc", reportBody(4, "x", staffScores()...)).status(t, 400)
	}},

	// ===== reports: delete =====
	{"reports: hapus wajib konfirmasi, lalu raport dan nilainya hilang", func(t *testing.T, c client) {
		id := int(num(t, c.do("POST", repP, reportBody(4, "Awal", staffScores()...)).status(t, 201).data()["id"]))
		path := fmt.Sprintf("%s/%d", repP, id)
		c.do("DELETE", path, nil).status(t, 400).hasErrorField(t, "confirm")
		c.do("DELETE", path+"?confirm=false", nil).status(t, 400)
		c.do("GET", path, nil).status(t, 200) // masih ada
		c.do("DELETE", path+"?confirm=true", nil).status(t, 200)
		c.do("GET", path, nil).status(t, 404)
		c.do("DELETE", path+"?confirm=true", nil).status(t, 404)
		c.do("DELETE", repP+"/abc?confirm=true", nil).status(t, 400)
		if n := len(c.do("GET", repP, nil).list()); n != 0 {
			t.Fatalf("daftar tidak kosong: %d", n)
		}
	}},
	{"reports: setelah dihapus fungsionaris bisa dibuatkan raport baru", func(t *testing.T, c client) {
		id := int(num(t, c.do("POST", repP, reportBody(4, "Lama", staffScores()...)).status(t, 201).data()["id"]))
		c.do("DELETE", fmt.Sprintf("%s/%d?confirm=true", repP, id), nil).status(t, 200)
		d := c.do("POST", repP, reportBody(4, "Baru", staffScores()...)).status(t, 201).data()
		if d["overall_evaluation"] != "Baru" {
			t.Fatalf("data = %v", d)
		}
	}},

	// ===== alur lengkap (sesuai demo) =====
	{"alur end-to-end admin", func(t *testing.T, c client) {
		users := c.do("GET", adm+"/users?role=staff", nil).status(t, 200).list()
		uid := int(num(t, users[0].(map[string]any)["id"]))
		aspects := c.do("GET", aspP+"?role_type=staff", nil).status(t, 200).list()
		var scores []sc
		for i, a := range aspects {
			scores = append(scores, sc{int(num(t, a.(map[string]any)["id"])), 70 + i*5})
		}
		created := c.do("POST", repP, reportBody(uid, "Alur lengkap", scores...)).status(t, 201).data()
		if num(t, created["average_score"]) != 80 { // (70+75+80+85+90)/5
			t.Fatalf("average = %v", created["average_score"])
		}
		id := int(num(t, created["id"]))
		c.do("POST", repP, reportBody(uid, "dup", scores...)).status(t, 409)
		c.do("PUT", fmt.Sprintf("%s/%d", repP, id), reportBody(uid, "Revisi", scores...)).status(t, 200)
		c.do("DELETE", fmt.Sprintf("%s/%d?confirm=true", repP, id), nil).status(t, 200)
	}},
}
