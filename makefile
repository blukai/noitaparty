CLIENT_CC ?= i686-w64-mingw32-gcc

client: 
	CGO_ENABLED=1 \
	GOOS=windows \
	GOARCH=386 \
	CC=$(CLIENT_CC) \
	go build -buildmode=c-shared -o ./mod/files/client.dll ./cmd/client/main.go

clean-client:
	rm ./mod/files/client.dll
	rm ./mod/files/client.h

server:
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build -o ./cmd/server/server ./cmd/server/main.go

clean-server:
	rm ./cmd/server/server

.PHONY: client clean-client server clean-server

