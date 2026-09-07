.PHONY: *

build:
	go build -o build/its main.go

package:
	docker buildx build -t its:dev --platform=linux/amd64 .

configure: build
	./build/its configure

sync: build
	./build/its sync

sync-container:
	test -f config.yaml || (echo "config.yaml not found; copy example.config.yaml to config.yaml first" && exit 1)
	touch trakt-token.json
	docker run -it --rm --platform=linux/amd64 --env-file=.env -v $(CURDIR)/config.yaml:/app/config.yaml:ro -v $(CURDIR)/trakt-token.json:/app/trakt-token.json its:dev

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
