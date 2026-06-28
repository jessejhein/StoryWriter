.PHONY: check backend-check frontend-check fmt test dev-backend dev-frontend

check: backend-check frontend-check

backend-check:
	go fmt ./...
	go vet ./...
	go test ./...

frontend-check:
	cd web && npm run lint && npm run typecheck && npm test -- --run && npm run build

fmt:
	go fmt ./...

test:
	go test ./...

dev-backend:
	go run ./cmd/storywork

dev-frontend:
	cd web && npm run dev
