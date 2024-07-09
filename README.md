# LUCIGO: A Go client for LUCIDAC

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
