SHELL = /bin/bash -u -e -o pipefail

# `make` applies env vars from `.env`
include .env

run:
	$(shell cat .env | egrep -v '^#' | xargs -0) \
	go run main.go

dev:
	which air || go install github.com/cosmtrek/air@latest
	$(shell cat .env | egrep -v '^#' | xargs -0) \
	air --build.delay=1000 \
		--build.cmd "go build -o main main.go" \
		--build.bin "./main"
