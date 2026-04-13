// pm2 ecosystem — two processes for claudemote
// Secrets (JWT_SECRET, ADMIN_PASSWORD_HASH) are NOT here;
// they are loaded by the Go binary from backend/.env at startup.
// Edit env values below before first deploy.

module.exports = {
  apps: [
    {
      name: "claudemote-api",
      cwd: "./backend",
      script: "./server",
      env: {
        PORT: 8080,
        WORKER_COUNT: 2,
        // Path to the git repo Claude Code should operate on. Default = the
        // claudemote install dir itself (self-hosting). Override by exporting
        // WORK_DIR in the shell before `pm2 start`.
        WORK_DIR: process.env.WORK_DIR || "/opt/atomiton/claudemote",
        // CLAUDE_BIN is read from backend/.env (auto-detected by start.sh bootstrap).
        CLAUDE_DEFAULT_MODEL: "claude-sonnet-4-6",
        CLAUDE_PERMISSION_MODE: "bypassPermissions",
        JOB_TIMEOUT_MIN: 30,
        MAX_COST_PER_JOB_USD: 1.0,
        JOB_LOG_RETENTION_DAYS: 14,
        DB_PATH: "/var/lib/claudemote/claudemote.db",
        // JWT_SECRET and ADMIN_PASSWORD_HASH are injected via backend/.env
      },
      max_memory_restart: "500M",
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
        PORT: 3000,
        // Internal server-side calls go direct; public calls use same-origin via Caddy
        BACKEND_URL: "http://localhost:8080",
        NEXT_PUBLIC_BACKEND_URL: "",
        // NextAuth v5 secret — operator must export AUTH_SECRET in shell before ./start.sh
        AUTH_SECRET: process.env.AUTH_SECRET,
        // NextAuth v5 also checks NEXTAUTH_SECRET as fallback
        NEXTAUTH_SECRET: process.env.AUTH_SECRET,
        NEXTAUTH_URL: process.env.NEXTAUTH_URL,
      },
      max_memory_restart: "500M",
      error_file: "./logs/web-err.log",
      out_file: "./logs/web-out.log",
      watch: false,
    },
  ],
};
