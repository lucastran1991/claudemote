-- Job logs table stores every line of stream-json output from Claude Code.
-- seq is the monotonically increasing line number within a job.
-- stream: stdout | stderr

CREATE TABLE IF NOT EXISTS job_logs (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id     TEXT     NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    seq        INTEGER  NOT NULL,
    stream     TEXT     NOT NULL DEFAULT 'stdout',
    line       TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Composite index supports ordered log retrieval and SSE replay.
CREATE INDEX IF NOT EXISTS idx_job_logs_job_id_seq ON job_logs(job_id, seq);
