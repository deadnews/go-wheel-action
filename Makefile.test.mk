.PHONY: test

test:
	GOWHEEL_VERSION=v0.0.1-alpha.0 \
	GOWHEEL_OUTPUT_DIR=./dist/wheel \
	GOWHEEL_PACKAGE=./cmd/go-wheel-action \
	go run ./cmd/go-wheel-action
