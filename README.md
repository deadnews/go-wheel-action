# go-wheel-action

> Package Go binaries as Python wheels

[![PyPI: Version](https://img.shields.io/pypi/v/go-wheel-action?logo=pypi&logoColor=white)](https://pypi.org/project/go-wheel-action)
[![GitHub: Release](https://img.shields.io/github/v/release/deadnews/go-wheel-action?logo=github&logoColor=white)](https://github.com/deadnews/go-wheel-action/releases/latest)
[![CI: Main](https://img.shields.io/github/actions/workflow/status/deadnews/go-wheel-action/main.yml?branch=main&logo=github&logoColor=white&label=main)](https://github.com/deadnews/go-wheel-action)
[![CI: Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/deadnews/go-wheel-action/refs/heads/badges/coverage.json)](https://github.com/deadnews/go-wheel-action)

**[Usage](#usage)** • **[Inputs](#inputs)** • **[CLI](#cli)** • **[Platforms](#platforms)**

## Usage

```yml
- uses: actions/setup-go@v6
  with:
    go-version-file: go.mod

- name: Build wheels
  uses: deadnews/go-wheel-action@v1
  with:
    package: ./cmd/myapp

- name: Publish to PyPI
  uses: pypa/gh-action-pypi-publish@v1
```

## Inputs

On a tag push, all inputs are optional — defaults are derived from the GitHub context.

| Input         | Default                | Description                                          |
| ------------- | ---------------------- | ---------------------------------------------------- |
| `mod-dir`     | `.`                    | Directory containing `go.mod`                        |
| `package`     | `.`                    | Go package to build (passed to `go build [package]`) |
| `version`     | `github.ref_name`      | Package version                                      |
| `name`        | basename of `mod-dir`  | Python package name and CLI command                  |
| `ldflags`     | `-s`                   | Go linker flags                                      |
| `output-dir`  | `./dist`               | Directory for built wheels                           |
| `readme`      | `README.md`            | Path to README for PyPI                              |
| `url`         | repository URL         | Project URL for PyPI                                 |
| `description` | repository description | Package summary for PyPI                             |
| `license`     | repository license     | License identifier for PyPI                          |

### CLI

Can be used outside of GitHub Actions via environment variables:

```sh
GOWHEEL_VERSION=0.0.1 \
GOWHEEL_PACKAGE=./cmd/myapp \
go run github.com/deadnews/go-wheel-action/cmd/go-wheel-action@latest

# or, since the tool publishes itself on PyPI:
GOWHEEL_VERSION=0.0.1 \
GOWHEEL_PACKAGE=./cmd/myapp \
uvx go-wheel-action
```

## Platforms

Builds wheels for 8 platform targets (all statically compiled with `CGO_ENABLED=0`):

| OS      | Arch  | Wheel tag                                         |
| ------- | ----- | ------------------------------------------------- |
| Linux   | amd64 | `manylinux_2_17_x86_64`, `musllinux_1_2_x86_64`   |
| Linux   | arm64 | `manylinux_2_17_aarch64`, `musllinux_1_2_aarch64` |
| macOS   | amd64 | `macosx_10_9_x86_64`                              |
| macOS   | arm64 | `macosx_11_0_arm64`                               |
| Windows | amd64 | `win_amd64`                                       |
| Windows | arm64 | `win_arm64`                                       |
