.PHONY: test

test:
	GOWHEEL_VERSION=0.0.0-test \
	GOWHEEL_OUTPUT_DIR=./dist/wheel \
	GOWHEEL_PACKAGE=./cmd/go-wheel-action \
	go run ./cmd/go-wheel-action
