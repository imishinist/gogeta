COMMIT=$(shell git rev-parse HEAD)
DATE=$(shell date +'%FT%TZ%z')

gogeta: generate
	go build -a -tags=netgo \
		-ldflags '-s -w -extldflags "-static" -X main.Commit=$(COMMIT) -X main.Date=$(DATE)'

.PHONY: generate
generate:
	go generate ./...

download:
	@echo Download go.mod dependencies
	@go mod download
 
install-tools: download
	@echo Installing tools from tools.go
	@cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %

.PHONY: clean
clean:
	rm -f gogeta
	rm -f *.pprof
