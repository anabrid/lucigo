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


build: build/lucigo $(shell find -iname \*.go -type f)
	cd cmd/lucigo && go build $(LDFLAGS) -o ../../build/ .
    GOOS=windows GOARCH=amd64 cd cmd/lucigo &&  go build $(LDFLAGS) -o ../../build/ . # Windows x86 build
#	term library currently doesnt (cross-)compile on mac
    #CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o build/ . # Mac OS X pre-arm64

install:
	cd cmd/lucigo
	cd cmd/lucigo && go install ${LDFLAGS}

test:
	go test .

clean:
	cd cmd/lucigo && go clean && rm -rf build/

LUCIGUI_PATH=cmd/lucigo/web-assets/lucigui
download_lucigui:
	mkdir -p $(LUCIGUI_PATH) && cd $(LUCIGUI_PATH)
	# assuming the zip has no subdirectory structure...
	wget https://github.com/anabrid/lucigui/releases/download/latest/lucigui-bundle.zip
	unzip lucigui-bundle.zip && rm lucigui-bundle.zip


.PHONY = install clean test
