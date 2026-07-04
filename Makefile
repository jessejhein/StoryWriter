GO := $(shell if [ -x /home/linuxbrew/.linuxbrew/bin/go ]; then printf '%s' /home/linuxbrew/.linuxbrew/bin/go; else printf '%s' go; fi)
export GOCACHE ?= $(CURDIR)/.gocache

.PHONY: check backend-check frontend-check race-check diff-check artifact-check fmt test dev-backend dev-frontend

check: backend-check frontend-check

backend-check:
	$(GO) fmt ./...
	$(GO) vet ./...
	$(GO) test ./...
	$(GO) test -race ./...
	git diff --check
	$(MAKE) artifact-check

race-check:
	$(GO) test -race ./...

diff-check:
	git diff --check

artifact-check:
	@tracked="$$(git ls-files | grep -E '(\.sqlite|\.db|\.db-shm|\.db-wal|\.bin|\.exe)$$|(^|/)(providers\.yaml|findings|prompts)(/|$$)' || true)"; \
	if [ -n "$$tracked" ]; then \
		printf '%s\n' "tracked artifact leak detected:" "$$tracked"; \
		exit 1; \
	fi

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
