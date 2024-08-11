default: guide

.PHONY: phone-auth email-auth

guide:
		echo "Plugin Development Guide"

apito-cdn:
		go build -buildmode=plugin -o local/apito-cdn-file-upload/main.so -gcflags all=-N -l local/apito-cdn-file-upload/main.go

local-upload:
		go build -buildmode=plugin -o local/go-local-func-execute-plugin/main.so -gcflags all=-N -l local/go-local-func-execute-plugin/main.go