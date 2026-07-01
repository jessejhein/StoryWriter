GO := $(shell if [ -x /home/linuxbrew/.linuxbrew/bin/go ]; then printf '%s' /home/linuxbrew/.linuxbrew/bin/go; else printf '%s' go; fi)
export GOCACHE ?= $(CURDIR)/.gocache

.PHONY: check backend-check frontend-check fmt test dev-backend dev-frontend

check: backend-check frontend-check

backend-check:
	$(GO) fmt ./...
	$(GO) vet ./...
	$(GO) test ./...

frontend-check:
	cd web && npm run lint && npm run typecheck && npm test -- --run && npm run build

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

dev-backend:
	$(GO) run ./cmd/storywork

dev-frontend:
	cd web && npm run dev
