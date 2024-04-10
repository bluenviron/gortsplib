bench:
	echo "$$DOCKERFILE_TEST" | docker build -q . -f - -t temp
	docker run --rm -it \
	--network=host \
	--name temp \
	temp \
	make bench-nodocker

bench-nodocker:
	go test -bench=. -run=^a -v ./pkg/...
