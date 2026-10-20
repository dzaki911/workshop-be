# Backend – Website Raport Evaluasi Fungsionaris Organisasi

Backend REST API untuk raport evaluasi BPH dan staff (Workshop Tugas 1 & 2 – role Backend).
Stack: Go (net/http, `database/sql` + pgx), PostgreSQL, arsitektur berlapis (domain → service → repository/delivery).

## Cara Menjalankan (Lokal)

Prasyarat: Go 1.22+ dan PostgreSQL.

```bash
# 1. Siapkan konfigurasi
cp .env.example .env            # lalu isi DB_PASSWORD dkk. (.env dibaca otomatis)

# 2. Buat database kosong
createdb -U postgres raport_evaluasi

# 3. Migrasi + seed akun uji
go run ./cmd/migrate up
go run ./cmd/seed

# 4. Jalankan server (http://localhost:8080)
go run ./cmd/api

# Test unit
go test ./...
```

### Migrasi database

File di `infra/database/migrations/` (`NNNNNN_nama.up.sql` / `.down.sql`, tersemat ke binary).
Versi yang sudah diterapkan dicatat di tabel `schema_migrations`; tiap migrasi berjalan dalam satu transaksi.

```bash
go run ./cmd/migrate up        # terapkan semua migrasi baru
go run ./cmd/migrate down      # batalkan 1 migrasi terakhir
go run ./cmd/migrate down 2    # batalkan 2 migrasi terakhir
```

### Seed akun uji (hanya lokal, `APP_ENV=development`)

`go run ./cmd/seed` bersifat idempotent. Password semua akun = `SEED_PASSWORD` (default `Password123!`).

| Email | Role |
|---|---|
| admin@raport.local | admin |
| bph@raport.local, bph2@raport.local | bph |
| staff@raport.local, staff2@raport.local | staff |

Plus 5 aspek BPH dan 5 aspek staff (contoh dari studi kasus). Seed menolak berjalan jika `APP_ENV` bukan `development`.

### Catatan autentikasi (sesi 2)

Auth/JWT dan middleware akses baru dibuat di sesi 3. Sementara itu, semua rute `/api/v1/admin/*`
memakai **admin uji** (`DEV_ADMIN_EMAIL`) tanpa header apa pun, dan hanya aktif jika `APP_ENV=development`
(di environment lain dijawab `401`). `created_by` pada raport selalu diambil dari admin ini, **bukan** dari body request
(field `created_by` di body diabaikan). Jangan expose API ini ke staging/publik sebelum sesi 3.

## Struktur Folder

```
cmd/api/        entrypoint server
cmd/migrate/    CLI migrasi (up/down)
cmd/seed/       seed akun & aspek uji
config/         konfigurasi dari env / .env
domain/         entity, error domain, interface Repository & Usecase
internal/
  service/              business logic & validasi (implementasi Usecase)
  repository/postgres/  query PostgreSQL (implementasi Repository)
  delivery/http/        handler, router, middleware
infra/database/         koneksi + runner migrasi + file SQL
pkg/response/           format response JSON seragam
docs/                   Postman collection
```

Arah dependensi: `delivery → domain ← service`, `repository → domain`; `domain` tidak mengimpor layer lain.

## Rancangan Database

Migrasi: [000001_init.up.sql](infra/database/migrations/000001_init.up.sql) / [down](infra/database/migrations/000001_init.down.sql).

| Tabel | Kolom |
|---|---|
| `users` | id, name, email (unique), password (hash bcrypt), role (`admin`/`bph`/`staff`), position, ministry, created_at, updated_at |
| `assessment_aspects` | id, name, description, role_type (`bph`/`staff`), created_at, updated_at — unique (name, role_type) |
| `reports` | id, user_id (**unique**, FK users), created_by (FK users), average_score (0–100), overall_evaluation, created_at, updated_at |
| `report_scores` | id, report_id (FK reports, cascade), aspect_id (FK assessment_aspects), score (CHECK 0–100) — unique (report_id, aspect_id) |

Relasi: `reports.user_id` → fungsionaris yang dinilai (satu raport per fungsionaris); `reports.created_by` → admin pembuat;
`report_scores` → banyak nilai per raport, satu per aspek. Kolom `snake_case`, primary key `BIGINT IDENTITY`.

## Kontrak Data (disepakati dengan FE)

- Rata-rata = **jumlah nilai ÷ jumlah aspek untuk role fungsionaris**, tanpa bobot, dihitung backend.
  Field response: `average_score` (angka, dibulatkan 2 desimal di backend; **FE menampilkan dengan `toFixed(2)`**, mis. `86.20`).
- Payload nilai: `scores: [{ "aspect_id": 6, "score": 85 }]`; `score` bilangan bulat 0–100.
- Satu raport per fungsionaris (`409` bila duplikat). Semua aspek role-nya wajib diberi nilai. Raport tanpa nilai ditolak.
- `user_id` pada `PUT` raport diabaikan (raport tidak dapat dipindah ke fungsionaris lain).
- Aspek yang sudah dipakai raport: tidak bisa dihapus dan `role_type`-nya tidak bisa diubah (`409`). Nama/deskripsi tetap bisa diubah.
- Menambah aspek baru setelah raport dibuat membuat raport lama belum lengkap terhadap aspek tsb; saat raport itu di-`PUT`, aspek baru wajib diisi.
- Hapus raport butuh `?confirm=true`, nilai terkait ikut terhapus.

### Format response

Sukses:
```json
{ "status": "success", "message": "Data successfully retrieved", "data": { "id": 1 } }
```
Error:
```json
{ "status": "error", "message": "Validation failed",
  "errors": [{ "field": "scores[0].score", "message": "Nilai harus 0–100" }] }
```

| Kode | Dipakai untuk |
|---|---|
| 200 / 201 | sukses / berhasil dibuat |
| 400 | validasi gagal, ID/JSON tidak valid |
| 401 | admin belum terautentikasi (di luar development) |
| 404 | data tidak ditemukan |
| 409 | raport duplikat, aspek terpakai, aspek duplikat |
| 500 | error server |
| 501 | endpoint auth (`/auth/*`, `/me*`) belum dibuat (sesi 3) |

## Endpoint Aktif (`/api/v1`)

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/health` | Cek status server |
| GET | `/admin/users?role=bph\|staff` | Daftar fungsionaris (BPH/staff) |
| GET | `/admin/aspects?role_type=bph\|staff` | Daftar aspek (filter opsional) |
| POST | `/admin/aspects` | Tambah aspek |
| GET | `/admin/aspects/:id` | Detail aspek |
| PUT | `/admin/aspects/:id` | Ubah aspek |
| DELETE | `/admin/aspects/:id` | Hapus aspek (409 bila terpakai) |
| GET | `/admin/reports` | Daftar raport |
| POST | `/admin/reports` | Buat raport + nilai (transaksi atomik) |
| GET | `/admin/reports/:id` | Detail raport + nilai per aspek |
| PUT | `/admin/reports/:id` | Ubah evaluasi & nilai (transaksi atomik) |
| DELETE | `/admin/reports/:id?confirm=true` | Hapus raport + nilai |

Sesi 3: `POST /auth/login`, `POST /auth/logout`, `GET /me`, `GET /me/report` (saat ini 501).

Aturan validasi raport (`internal/service/report.go`): fungsionaris wajib & harus bph/staff; evaluasi wajib (bukan spasi saja);
nilai 0–100; aspek harus sesuai role dan tidak duplikat; semua aspek role wajib bernilai; minimal satu nilai; satu raport per fungsionaris.

## Pengujian Lokal

1. Import [docs/postman_collection.json](docs/postman_collection.json) ke Postman (variabel `baseUrl` = `http://localhost:8080`).
2. Pastikan migrasi + seed sudah dijalankan, lalu jalankan server. Urutan demo: daftar users → daftar aspek → buat raport
   (id aspek staff 6–10 pada DB baru) → coba nilai 101 / evaluasi kosong / buat ulang (409) → ubah → hapus aspek terpakai (409) → hapus raport `?confirm=true`.
3. Contoh curl:

```bash
curl http://localhost:8080/api/v1/admin/aspects?role_type=staff
curl -X POST http://localhost:8080/api/v1/admin/reports -H "Content-Type: application/json" \
  -d '{"user_id":4,"overall_evaluation":"Baik","scores":[{"aspect_id":6,"score":85}]}'
```

## Automated Test

```bash
go test ./...
```

Tanpa database, ini menjalankan:
- `internal/service`: unit test validasi raport dan perhitungan rata-rata.
- `internal/delivery/http/api_test.go`: test seluruh endpoint (status, format response, validasi, 400/401/404/409/501,
  alur lengkap) terhadap repository in-memory (`internal/repository/memory`).

Suite endpoint yang sama bisa dijalankan terhadap PostgreSQL asli dengan mengisi `TEST_DATABASE_URL`.
**Awas:** database tersebut di-reset (migrate down lalu up) di setiap test, jadi pakai database khusus test, bukan `raport_evaluasi`.

```bash
createdb -U postgres raport_test
# PowerShell:
$env:TEST_DATABASE_URL = "postgres://postgres:PASSWORD@localhost:5432/raport_test?sslmode=disable"
go test -count=1 ./internal/delivery/http
```

## Pembagian Tugas

| Anggota | Tugas |
|---|---|
| Musthafa Dzaki Nugraha | Backend: setup repo, skema & migrasi, arsitektur berlapis, CRUD aspek & raport, validasi, dokumentasi |
