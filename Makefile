APP-BIN := ./bin/alertmatter.bin

LAST_COMMIT := $(shell git rev-parse --short HEAD)
LAST_COMMIT_DATE := $(shell git show -s --format=%ci ${LAST_COMMIT})
VERSION := $(shell git describe --tags)
BUILDSTR := ${VERSION} (Commit: ${LAST_COMMIT_DATE} (${LAST_COMMIT}), Build: $(shell date +"%Y-%m-%d% %H:%M:%S %z"))

.PHONY: build
build: ## Build binary.
	CGO_ENABLED=0 go build -o ${APP-BIN} -ldflags="-X 'main.buildString=${BUILDSTR}'" .

.PHONY: run
run: ## Run binary.
	./${APP-BIN}

.PHONY: clean
clean: ## Remove temporary files and the `bin` folder.
	rm -rf bin

.PHONY: fresh
fresh: build run

.PHONY: lint
lint:
	docker run --rm -v $(pwd):/app -w /app golangci/golangci-lint:v1.43.0 golangci-lint run -v
