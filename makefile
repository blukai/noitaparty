GOOS ?= windows
GOARCH ?= 386
CC_FOR_TARGET ?= i686-w64-mingw32-gcc

client: 
	CGO_ENABLED=1 \
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	CC=$(CC_FOR_TARGET) \
	go build -buildmode=c-shared -o ./mod/files/client.dll ./cmd/client/main.go

clean:
	rm -rf ./mod/files/client.dll
	rm -rf ./mod/files/client.h

.PHONY: client clean

