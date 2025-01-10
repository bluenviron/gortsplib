lint:
	docker run --rm -v $(shell pwd):/app -w /app \
	-e CGO_ENABLED=0 \
	$(LINT_IMAGE) \
	golangci-lint run -v
