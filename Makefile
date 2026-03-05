BINARY := wakeup-service

.PHONY: build build-arm64 build-armv7 clean

## Build for the current platform
build:
	go build -o $(BINARY) .

## Cross-compile for Raspberry Pi 3 / 4 / 5 (64-bit)
build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BINARY)-arm64 .

## Cross-compile for Raspberry Pi 2 / Zero / Zero W (32-bit ARMv7)
build-armv7:
	GOOS=linux GOARCH=arm GOARM=7 go build -o $(BINARY)-armv7 .

clean:
	rm -f $(BINARY) $(BINARY)-arm64 $(BINARY)-armv7
