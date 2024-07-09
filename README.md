# LUCIGO: A Go client for LUCIDAC

This repository holds a little client for the LUCIDAC with the focus on
*device administration*. While all other clients available require ecosystems
such as Python PIP or the Julia package manager, the LUCIGO client can be
easily compiled and distributed as static binaries for all major desktop
platforms. This makes it great for *getting started*, in particular for novices
who cannot do well with the terminal.

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

Install go with your package manager, such as `apt install golang`, or follow
the instructions from https://go.dev/doc/install

A command like `go run . --help` or `go build . && ./lucigo --help` will get
you compiled and started. Cross compilation is very easy, for instance
`GOOS="windows" GOARCH="amd64" go build .`. Current executable sizes/artifact
sizes are about 15MB in size.

## License

As all anabrid code within the LUCIDAC project, this code is dual licensed:

> Copyright (c) 2024 anabrid GmbH
> 
> Contact: https://www.anabrid.com/licensing/
>
> SPDX-License-Identifier: MIT OR GPL-2.0-or-later