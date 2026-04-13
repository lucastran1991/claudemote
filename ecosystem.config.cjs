// pm2 ecosystem — two processes for claudemote
// Non-secret config is read from system.cfg.json at the project root.
// Secrets (JWT_SECRET, ADMIN_PASSWORD_HASH) are NOT here;
// they are loaded by the Go binary from backend/.env at startup.

const cfg = require('./system.cfg.json');

module.exports = {
  apps: [
    {
      name: "claudemote-api",
      cwd: "./backend",
      script: "./server",
      env: {
        PORT: cfg.api.port,
        WORKER_COUNT: cfg.worker.count,
        // Path to the git repo Claude Code should operate on. Default = the
        // claudemote install dir itself (self-hosting). Override by exporting
        // WORK_DIR in the shell before `pm2 start`.
        WORK_DIR: process.env.WORK_DIR || cfg.work_dir,
        // CLAUDE_BIN is read from backend/.env (auto-detected by start.sh bootstrap).
        CLAUDE_DEFAULT_MODEL: cfg.worker.model,
        CLAUDE_PERMISSION_MODE: cfg.worker.permission_mode,
        JOB_TIMEOUT_MIN: cfg.jobs.timeout_min,
        MAX_COST_PER_JOB_USD: cfg.jobs.max_cost_usd,
        JOB_LOG_RETENTION_DAYS: cfg.jobs.log_retention_days,
        DB_PATH: cfg.db_path,
        CORS_ORIGIN: `http://localhost:${cfg.web.port}`,
        // JWT_SECRET and ADMIN_PASSWORD_HASH are injected via backend/.env
      },
      max_memory_restart: cfg.pm2_max_memory,
      error_file: "./logs/api-err.log",
      out_file: "./logs/api-out.log",
      watch: false,
    },
    {
      name: "claudemote-web",
      cwd: "./frontend",
      script: "pnpm",
      args: "start",
      env: {
        PORT: cfg.web.port,
        // Internal server-side calls go direct; public calls use same-origin via Caddy
        BACKEND_URL: `http://localhost:${cfg.api.port}`,
        NEXT_PUBLIC_BACKEND_URL: "",
        // NextAuth v5 secret — operator must export AUTH_SECRET in shell before ./start.sh
        AUTH_SECRET: process.env.AUTH_SECRET,
        // NextAuth v5 also checks NEXTAUTH_SECRET as fallback
        NEXTAUTH_SECRET: process.env.AUTH_SECRET,
        NEXTAUTH_URL: process.env.NEXTAUTH_URL,
        AUTH_TRUST_HOST: "true",
      },
      max_memory_restart: cfg.pm2_max_memory,
      error_file: "./logs/web-err.log",
      out_file: "./logs/web-out.log",
      watch: false,
    },
  ],
};
