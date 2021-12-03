
BASE_IMAGE = golang:1.17-alpine3.14
LINT_IMAGE = golangci/golangci-lint:v1.40

.PHONY: $(shell ls)

help:
	@echo "usage: make [action]"
	@echo ""
	@echo "available actions:"
	@echo ""
	@echo "  mod-tidy       run go mod tidy"
	@echo "  format         format source files"
	@echo "  test           run tests"
	@echo "  lint           run linter"
	@echo "  bench          run benchmarks"
	@echo ""

blank :=
define NL

$(blank)
endef

mod-tidy:
	docker run --rm -it -v $(PWD):/s -w /s $(BASE_IMAGE) \
	sh -c "apk add git && GOPROXY=direct go get && go mod tidy"

define DOCKERFILE_FORMAT
FROM $(BASE_IMAGE)
RUN go install mvdan.cc/gofumpt@v0.2.0
endef
export DOCKERFILE_FORMAT

format:
	echo "$$DOCKERFILE_FORMAT" | docker build -q . -f - -t temp
	docker run --rm -it -v $(PWD):/s -w /s temp \
	sh -c "gofumpt -l -w ."

define DOCKERFILE_TEST
FROM $(BASE_IMAGE)
RUN apk add --no-cache make docker-cli git gcc musl-dev
WORKDIR /s
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
endef
export DOCKERFILE_TEST

test:
	echo "$$DOCKERFILE_TEST" | docker build -q . -f - -t temp
	docker run --rm -it \
	-v /var/run/docker.sock:/var/run/docker.sock:ro \
	--network=host \
	--name temp \
	temp \
	make test-nodocker

test-examples:
	go build -o /dev/null ./examples/...

test-pkg:
	go test -v -race -coverprofile=coverage-pkg.txt ./pkg/...

test-root:
	$(foreach IMG,$(shell echo testimages/*/ | xargs -n1 basename), \
	docker build -q testimages/$(IMG) -t gortsplib-test-$(IMG)$(NL))
	go test -v -race -coverprofile=coverage-root.txt .

test-nodocker: test-examples test-pkg test-root

lint:
	docker run --rm -v $(PWD):/app -w /app \
	$(LINT_IMAGE) \
	golangci-lint run -v

bench:
	echo "$$DOCKERFILE_TEST" | docker build -q . -f - -t temp
	docker run --rm -it \
	--network=host \
	--name temp \
	temp \
	make bench-nodocker

bench-nodocker:
	go test -bench=. -v ./pkg/ringbuffer
