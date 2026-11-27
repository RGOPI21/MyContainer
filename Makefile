.PHONY:	all	build-c	build-go clean

all: build-c build-go

build-c:
		gcc -O2 runtime/main.c -o my-container-runtime

build-go:
		go build -o my-tool .

clean:
		rm -f my-container-runtime my-tool
		-sudo umount -l /tmp/my-containers/*/rootfs 2>/dev/null
		sudo rm -rf /tmp/my-containers/*
