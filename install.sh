#!/bin/sh
# Installs vegadesk by building it from source with `go install`, then
# copying the resulting binary onto PATH. Installs a Go toolchain first if
# one isn't already present. POSIX sh so it works piped straight into `sh`
# regardless of the user's shell.
set -eu

REPO="github.com/sabbakix/vegadesk"
BIN_NAME="vegadesk"
BIN_DIR="/usr/local/bin"

info() { printf '%s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

as_root() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	else
		command -v sudo >/dev/null 2>&1 || die "need root (or sudo) to $*"
		sudo "$@"
	fi
}

install_go() {
	if command -v apt-get >/dev/null 2>&1; then
		as_root apt-get update
		as_root apt-get install -y golang-go
	elif command -v dnf >/dev/null 2>&1; then
		as_root dnf install -y golang
	elif command -v yum >/dev/null 2>&1; then
		as_root yum install -y golang
	elif command -v pacman >/dev/null 2>&1; then
		as_root pacman -Sy --noconfirm go
	elif command -v zypper >/dev/null 2>&1; then
		as_root zypper install -y go
	elif command -v apk >/dev/null 2>&1; then
		as_root apk add --no-cache go
	else
		die "no supported package manager found; install Go 1.21+ yourself from https://go.dev/dl/ and re-run this script"
	fi
}

if ! command -v go >/dev/null 2>&1; then
	info "Go not found, installing it first..."
	install_go
fi

info "Fetching and building vegadesk from $REPO (may also update the Go toolchain, per go.mod)..."
go install "$REPO/cmd/$BIN_NAME@latest"

GOBIN="$(go env GOBIN)"
[ -n "$GOBIN" ] || GOBIN="$(go env GOPATH)/bin"
[ -x "$GOBIN/$BIN_NAME" ] || die "build finished but $GOBIN/$BIN_NAME is missing"

info "Installing to $BIN_DIR/$BIN_NAME..."
as_root install -m755 "$GOBIN/$BIN_NAME" "$BIN_DIR/$BIN_NAME"

info "Done -- run '$BIN_NAME' from any directory."
