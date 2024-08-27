lint:
	docker run --rm -v $(PWD):/app -w /app \
	-e CGO_ENABLED=0 \
	$(LINT_IMAGE) \
	golangci-lint run -v
