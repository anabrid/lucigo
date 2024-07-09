

VERSION=$(shell git describe --tags)
BUILD=$(shell git rev-parse --short HEAD)

#BUNDLE_GUI=True

XFLAGS= -X main.Version=${VERSION}
XFLAGS+=-X main.Build=${BUILD}
ifdef ${BUNDLE_GUI}
XFLAGS+=-X main.lucigui_bundled=true
else
XFLAGS+=-X main.lucigui_bundled=false
endif

LDFLAGS=-ldflags="${XFLAGS}"

build: $(wildcard *.go)
	go build ${LDFLAGS} -o build/ .
    GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o build/ . # Windows x86 build
#	term library currently doesnt (cross-)compile on mac
    #CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o build/ . # Mac OS X pre-arm64

install:
	go install ${LDFLAGS}

test:
	go test .

clean:
	go clean
	rm -rf build/

.PHONY = install clean test