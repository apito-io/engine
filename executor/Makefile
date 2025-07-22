default: revert

.PHONY: build revert

executor:
	find ./ -type f -name "*.go" -exec sed -i -e 's/package executor/package main/g' {} \; && \
	go build -buildmode=plugin -o executor.so . && \
	make revert

revert:
	find ./ -type f -name "*.go" -exec sed -i -e 's/package main/package executor/g' {} \;