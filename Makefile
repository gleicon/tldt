.PHONY: build test install clean

build:
	go build ./cmd/tldt

test:
	go test ./...

install:
	go install ./cmd/tldt

clean:
	rm -f tldt
