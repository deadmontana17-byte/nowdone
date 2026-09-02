# Project: Daily Planner with Telegram Bot

## Tech Stack
- Frontend: React 18, TypeScript, Vite, Material UI, Zustand, React Query
- Backend: Go 1.22, Gin framework, PostgreSQL 15, pgx
- Infrastructure: Docker Compose, Caddy for HTTPS

## Architecture
- Backend: layered architecture (handlers → services → repository)
- Frontend: feature-based structure (components/, pages/, store/, api/)
- All secrets in .env, never hardcode

## Coding Conventions
- Go: follow standard gofmt, use context for all DB operations
- TypeScript: use interfaces for all props, strict mode enabled
- Commit messages: conventional commits

## Project Goals
- Generate complete, production-ready code
- Include Dockerfiles and docker-compose.yml
- Provide README with deployment instructions