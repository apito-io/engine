default: guide

.PHONY: phone-auth email-auth

guide:
	echo "Plugin Development Guide"

update:
	go get -u github.com/apito-cms/buffers@o-production

docker:
	docker build --platform=linux/amd64 -t fahim/plugin-builder:v1 .

build:
	docker container run --entrypoint='' \
        --platform linux/amd64 \
        -v .:/usr/src/app \
        -w /usr/src/app \
        fahim/plugin-builder:v1 /bin/sh -c 'env GOOS=linux go build -buildmode=plugin -o main.so main.go'

build-local:
	go build -buildmode=plugin -o main.so -gcflags='all=-N -l' .