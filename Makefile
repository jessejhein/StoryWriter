GO := $(shell if [ -x /home/linuxbrew/.linuxbrew/bin/go ]; then printf '%s' /home/linuxbrew/.linuxbrew/bin/go; else printf '%s' go; fi)
export GOCACHE ?= $(CURDIR)/.gocache

.PHONY: check backend-check frontend-check race-check diff-check artifact-check tool-check fmt test dev-backend dev-frontend

check: backend-check frontend-check

backend-check:
	$(GO) fmt ./...
	$(GO) vet ./...
	$(GO) test ./...
	$(GO) test -race ./...
	git diff --check
	$(MAKE) tool-check
	$(MAKE) artifact-check

tool-check:
	python3 -m unittest tools.milestone_loop.test_codex_milestone_loop

race-check:
	$(GO) test -race ./...

diff-check:
	git diff --check

artifact-check:
	@files="$$( { git ls-files; git ls-files --others --exclude-standard; } | grep -vE '^(\\.gocache/|web/node_modules/|web/dist/|web/coverage/|\\.uv-cache/|\\.uv-python/)' || true)"; \
	leaks="$$(printf '%s\n' "$$files" | grep -E '(^|/)\.git$$|(^|/)(providers\.yaml|findings|prompts)(/|$$)|(\.sqlite|\.db|\.db-shm|\.db-wal|\.bin|\.exe|\.lock)$$' || true)"; \
	if [ -n "$$leaks" ]; then \
		printf '%s\n' "artifact leak detected:" "$$leaks"; \
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
