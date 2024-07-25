# LUCIGO: A Go client for LUCIDAC

[![Go Reference](https://pkg.go.dev/badge/github.com/anabrid/lucigo.svg)](https://pkg.go.dev/github.com/anabrid/lucigo)
[![Github CI](https://github.com/anabrid/lucigo/actions/workflows/go.yml/badge.svg)](https://github.com/anabrid/lucigo)

This repository holds a little client for the LUCIDAC with the focus on
*device administration*. While all other clients available require ecosystems
such as Python PIP or the Julia package manager, the LUCIGO client can be
easily compiled and distributed as static binaries for all major desktop
platforms. This makes it great for *getting started*, in particular for novices
who cannot do well with the terminal.

## Getting started as a user

Just grab the [latest release](https://github.com/anabrid/lucigo/releases).
It contains binaries ready to download for all major platforms (Mac OS X,
MS Windows, GNU/Linux).

For instance, on Linux you get started with these three commands

```
wget https://github.com/anabrid/lucigo/releases/download/latest/lucigo
chmod 755 lucigo
./lucigo --help
```

## Scope

The current scope of this client implementation is not to provide a full
experience of configuring and running analog circuits but instead on changing
permanent settings and bringing LUCIDAC onto the network. Typical use cases are

- User needs USB to bootstrap the networking, i.e. set some static IPv4 on
  the device before it can start.
- User cannot make use of networking but USB works. He nevertheless wants to
  enjoy the web based GUI. As the embedded webserver is not reachable, lucigo
  is ready to provide a (websocket'ing) webserver and proxy the USB Serial.

## Feature list

Checked if implemented

- [x] USB communication
- [x] TCP/IP communication
- [x] mDNS discovery
- [x] basic CLI
- [x] convenient permanent settings (hierarchical and shorthanded)
- [x] websocket proxying
- [ ] USB Serial discovery
- [ ] current serial USB library does not work for Mac (at least not cross compiling, cf the gitlab-CI)

## Alternatives

Mangling and forwarding streams is something various tools can perform, for instance:

* [Websockify](https://github.com/novnc/websockify) as a Websocket-TCP proxy/bridge
* [socat](http://www.dest-unreach.org/socat/doc/socat.html) or
  [goproxy](https://github.com/snail007/goproxy) for general purpose network proxying
  for TCP/UDP/websocket services (see in particular
  an [example how to use socat to redirect a device socket](https://bloggerbust.ca/post/let-socket-cat-be-thy-glue-over-serial/)
* [pyserial's `tcp_serial_redirect.py`](https://raw.githubusercontent.com/pyserial/pyserial/master/examples/tcp_serial_redirect.py)
  as a standalone code for tcp-serial redirection (also featured
  [within the pyanalog docs](https://www.anabrid.dev/docs/pyanalog/dirhtml/hycon/networking/)).
* [jq](https://github.com/jqlang/jq) as a command line JSON query processor and beautifier
* For zeroconf lookup, there are various clients such as `avahi-discover` (Linux),
  `dns-sd -B _lucijsonl._tcp` (Mac) or the cross-platform
  [async_browser.py](https://github.com/python-zeroconf/python-zeroconf/blob/master/examples/async_browser.py).


Obviously, these tools do not speak the LUCIDAC protocol (well, some speak JSON, thought).
Lucigo is merely a single interface for some features provided by much powerful tools
above. Lucigo comes without dependencies and installation hazzle, which makes usage very
simple.


## Getting started as developer

Once Go is running on your system, it is very easy to build the code. Here are
three possible methods:

1. Just run `go install github.com/anabrid/lucigo@latest` from somewhere, without
   checking out the repository or anything. This will download and compile all
   dependencies and put them to `$GOPATH`, which defaults to `~/go/bin` on
   Linux and Mac.
2. Check out the repository, run `go build .` and find the executable in the
   same directory.
3. Check out the repository, run `make build`, find the executables in the `build/`
   directory. This also does cross compilation, so you find executables for all
   platforms in the `build/` directory. Choose yours.

Note that using `make` is the currently the only way of building a fully fledged 
executable which has version information about itself (`lucigo --version`) and
contains a build image of the LUCIGUI. The other ways of building the executable
are still fine and working, however, for instance, `lucigo --version` currently
holds no information in these builds.

Current executable sizes/artifact sizes are about 15MB in size.

### Installing Go
Install go with your package manager, such as `apt install golang`, or follow
the instructions from https://go.dev/doc/install

On Mac, need to make sure that Xcode is installed before you can use the go
compiler, otherwisse you will get an error about `xcrun`. Run
`xcode-select --install` and also probably follow the steps
[described here](https://stackoverflow.com/questions/52522565/git-is-not-working-after-macos-update-xcrun-error-invalid-active-developer-p#52522566)
if the xcrun problems remain.

## License

As all anabrid code within the LUCIDAC project, this code is dual licensed:

> Copyright (c) 2024 anabrid GmbH
> 
> Contact: https://www.anabrid.com/licensing/
>
> SPDX-License-Identifier: MIT OR GPL-2.0-or-later

## Go infrastructure

Manually trigger an update at https://pkg.go.dev/github.com/anabrid/lucigo with:

```
VER=$(git describe --tags)
curl https://sum.golang.org/lookup/github.com/anabrid/lucigo@$VER
```
