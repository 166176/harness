.PHONY: test build webui docker
test:
	go test ./...
build: webui
	go build -o bin/gavel ./cmd/gavel
webui:
	cd webui && npm ci && npm run build
docker:
	docker build -t ghcr.io/166176/harness:latest .
