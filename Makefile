VERSION=$(shell git describe --tags)
BUILD=$(shell git rev-parse --short HEAD)

#BUNDLE_GUI=True

XFLAGS= -X main.Version=${VERSION}
XFLAGS+=-X main.Build=${BUILD}
ifdef BUNDLE_GUI
XFLAGS+=-X main.lucigui_bundled=true
else
XFLAGS+=-X main.lucigui_bundled=false
endif

LDFLAGS=-ldflags="$(XFLAGS)"


build: build-native build-win build-mac build-linux

build-any:
	cd cmd/lucigo && go build $(LDFLAGS) -o ../../build/$(OUTFILENAME) .

build-native:
	make build-any OUTFILENAME=lucigo-native

build-linux:
	make build-any GOOS=linux GOARCH=amd64 OUTFILENAME=lucigo-amd64-linux

build-win:
	make build-any GOOS=windows GOARCH=amd64 OUTFILENAME=lucigo-amd64-win.exe

build-mac:
	# amd64 covers also pre-amd64 while M1 arm is not backwards compatible
	make build-any GOOS=darwin GOARCH=amd64 OUTFILENAME=lucigo-amd64-mac

install:
	cd cmd/lucigo
	cd cmd/lucigo && go install ${LDFLAGS}

test:
	go test .

clean:
	rm -rf build/
	cd cmd/lucigo && go clean

LUCIGUI_PATH=cmd/lucigo/web-assets/lucigui
download_lucigui:
	mkdir -p $(LUCIGUI_PATH) && cd $(LUCIGUI_PATH)
	# assuming the zip has no subdirectory structure...
	wget https://github.com/anabrid/lucigui/releases/download/latest/lucigui-bundle.zip
	unzip lucigui-bundle.zip && rm lucigui-bundle.zip


.PHONY: install clean test build-any download_lucigui
