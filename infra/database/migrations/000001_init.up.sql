-- Skema awal: Website Raport Evaluasi Fungsionaris (PostgreSQL)

CREATE TYPE user_role AS ENUM ('admin', 'bph', 'staff');
CREATE TYPE aspect_role_type AS ENUM ('bph', 'staff');

CREATE TABLE users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    email      VARCHAR(150) NOT NULL UNIQUE,
    password   VARCHAR(255) NOT NULL,            -- hash bcrypt
    role       user_role    NOT NULL,
    position   VARCHAR(100),                     -- jabatan, opsional
    ministry   VARCHAR(100),                     -- kementerian/departemen, opsional
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE assessment_aspects (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        VARCHAR(100)     NOT NULL,
    description TEXT             NOT NULL DEFAULT '',
    role_type   aspect_role_type NOT NULL,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    UNIQUE (name, role_type)
);

CREATE TABLE reports (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id            BIGINT       NOT NULL UNIQUE REFERENCES users(id),   -- 1 fungsionaris = 1 raport
    created_by         BIGINT       NOT NULL REFERENCES users(id),          -- admin pembuat
    average_score      NUMERIC(5,2) NOT NULL CHECK (average_score BETWEEN 0 AND 100),
    overall_evaluation TEXT         NOT NULL CHECK (length(btrim(overall_evaluation)) > 0),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE report_scores (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_id BIGINT   NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    aspect_id BIGINT   NOT NULL REFERENCES assessment_aspects(id),
    score     SMALLINT NOT NULL CHECK (score BETWEEN 0 AND 100),
    UNIQUE (report_id, aspect_id)
);

CREATE INDEX idx_aspects_role_type ON assessment_aspects(role_type);
CREATE INDEX idx_report_scores_aspect ON report_scores(aspect_id);
