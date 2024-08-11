default: build

.PHONY: pull-buffer build

pull-buffer:
	go get -u github.com/apito-io/buffers@main

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s" -a -installsuffix cgo .