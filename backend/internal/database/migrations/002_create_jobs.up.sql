-- Jobs table tracks every Claude Code invocation.
-- status values: pending | running | done | failed | cancelled

CREATE TABLE IF NOT EXISTS jobs (
    id              TEXT     PRIMARY KEY,  -- uuid
    command         TEXT     NOT NULL,
    model           TEXT     NOT NULL,
    status          TEXT     NOT NULL DEFAULT 'pending',
    exit_code       INTEGER,              -- nullable
    summary         TEXT     NOT NULL DEFAULT '',
    session_id      TEXT     NOT NULL DEFAULT '',
    duration_ms     INTEGER  NOT NULL DEFAULT 0,
    total_cost_usd  REAL     NOT NULL DEFAULT 0.0,
    num_turns       INTEGER  NOT NULL DEFAULT 0,
    is_error        INTEGER  NOT NULL DEFAULT 0,  -- SQLite boolean
    stop_reason     TEXT     NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME,             -- nullable
    finished_at     DATETIME             -- nullable
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
