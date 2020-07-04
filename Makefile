
.PHONY: $(shell ls)

BASE_IMAGE = amd64/golang:1.14-alpine3.12

help:
	@echo "usage: make [action]"
	@echo ""
	@echo "available actions:"
	@echo ""
	@echo "  mod-tidy       run go mod tidy"
	@echo "  format         format source files"
	@echo "  test           run available tests"
	@echo ""

mod-tidy:
	docker run --rm -it -v $(PWD):/s $(BASE_IMAGE) \
	sh -c "apk add git && cd /s && go get && go mod tidy"

format:
	docker run --rm -it -v $(PWD):/s $(BASE_IMAGE) \
	sh -c "cd /s && find . -type f -name '*.go' | xargs gofmt -l -w -s"

define DOCKERFILE_TEST
FROM $(BASE_IMAGE)
RUN apk add --no-cache make git
WORKDIR /s
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
endef
export DOCKERFILE_TEST

test:
	echo "$$DOCKERFILE_TEST" | docker build -q . -f - -t temp
	docker run --rm -it \
	--name temp \
	temp \
	make test-nodocker

IMAGES = $(shell echo test-images/*/ | xargs -n1 basename)

test-nodocker:
	$(eval export CGO_ENABLED = 0)
	go test -v .
