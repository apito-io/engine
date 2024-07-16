BINARY_NAME=hello-world-echo
VERSION=$(shell git describe --tags --abbrev=0)
LDFLAGS=-w -s

build:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME) -ldflags "$(LDFLAGS)"

zip:
    zip $(BINARY_NAME)-$(VERSION).zip $(BINARY_NAME)

release: build zip
    gh release create $(VERSION) $(BINARY_NAME)-$(VERSION).zip
