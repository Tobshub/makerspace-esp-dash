-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    name text NOT NULL,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sessions_token_hash_unique UNIQUE (token_hash)
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    slug text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT teams_slug_unique UNIQUE (slug)
);

CREATE TABLE team_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id uuid NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT team_members_team_user_unique UNIQUE (team_id, user_id),
    CONSTRAINT team_members_role_check CHECK (role IN ('owner', 'admin', 'member', 'viewer'))
);

CREATE INDEX team_members_user_id_idx ON team_members (user_id);

CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id uuid NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    name text NOT NULL,
    slug text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT projects_team_slug_unique UNIQUE (team_id, slug)
);

CREATE INDEX projects_team_id_idx ON projects (team_id);

-- +goose Down
DROP TABLE projects;
DROP TABLE team_members;
DROP TABLE teams;
DROP TABLE sessions;
DROP TABLE users;
