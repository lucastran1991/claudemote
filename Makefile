.PHONY: dev build test migrate-up create-admin deploy reload logs clean

# Run both servers locally in development mode (not via pm2)
dev:
	( cd backend && go run ./cmd/server ) & \
	( cd frontend && pnpm dev )

# Compile Go binary + Next.js production build
build:
	( cd backend && go build -o server ./cmd/server )
	( cd frontend && pnpm install --frozen-lockfile && pnpm build )

# Run all tests
test:
	( cd backend && go test ./... )
	( cd frontend && pnpm test )

# Apply database migrations
migrate-up:
	( cd backend && go run ./cmd/server -migrate-only )

# Create the initial admin user (run once after first deploy)
create-admin:
	( cd backend && go run ./cmd/create-admin )

# Full deploy: rebuild then reload pm2 (no downtime on binary reload)
deploy: build reload

# Reload pm2 processes without full rebuild (pick up already-built artifacts)
reload:
	pm2 reload ecosystem.config.cjs

# Tail all pm2 logs
logs:
	pm2 logs

# Remove compiled artifacts
clean:
	rm -f backend/server
	rm -rf frontend/.next
