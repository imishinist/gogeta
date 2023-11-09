COMMIT=$(shell git rev-parse HEAD)
DATE=$(shell date +'%FT%TZ%z')

gogeta:
	go build -a -tags=netgo \
		-ldflags '-s -w -extldflags "-static" -X main.Commit=$(COMMIT) -X main.Date=$(DATE)'

.PHONY: clean
clean:
	rm gogeta
