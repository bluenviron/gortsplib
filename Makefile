BASE_IMAGE = golang:1.17-alpine3.14
LINT_IMAGE = golangci/golangci-lint:v1.49.0

.PHONY: $(shell ls)

help:
	@echo "usage: make [action]"
	@echo ""
	@echo "available actions:"
	@echo ""
	@echo "  mod-tidy        run go mod tidy"
	@echo "  format          format source files"
	@echo "  test            run tests"
	@echo "  test-highlevel  run high-level tests"
	@echo "  lint            run linter"
	@echo "  bench           run benchmarks"
	@echo ""

blank :=
define NL

$(blank)
endef

include scripts/*.mk
