.PHONY: run test

run:
	go run cmd/bot/main.go

test:
	go test -v ./internal/...
