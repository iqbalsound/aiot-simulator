# Handy shortcuts. Everything also works with plain `go` commands.
VERSION := $(shell cat VERSION)

.PHONY: run test vet fmt build package clean
run:      ## dry run from source (opens the simulator, nothing is installed)
	go run ./cmd/aiot-simulator
test:     ## unit + API tests with the race detector
	CGO_ENABLED=1 go test -race ./...
vet:
	go vet ./...
fmt:      ## fail if any file needs gofmt
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
build:    ## native binary in ./build/
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o build/aiot-simulator ./cmd/aiot-simulator
package:  ## DEB + RPM + Windows installer in ./dist/ (see build.sh for tool requirements)
	./build.sh $(VERSION)
clean:
	rm -rf build dist
