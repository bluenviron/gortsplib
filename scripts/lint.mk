lint-go:
	docker run --rm -v $(shell pwd):/app -w /app \
	-e CGO_ENABLED=0 \
	$(LINT_IMAGE) \
	golangci-lint run -v

lint-other:
	echo "$$DOCKERFILE_PRETTIER" | docker build . -f - -t temp
	docker run --rm -v "$(shell pwd)/:/s" -w /s temp \
	sh -c "prettier --check ."

lint: lint-go lint-other
