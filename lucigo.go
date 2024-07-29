// Copyright (c) 2024 anabrid GmbH
// Contact: https://www.anabrid.com/licensing/
// SPDX-License-Identifier: MIT OR GPL-2.0-or-later

/*
Package lucigo provides a client for the LUCIDAC analog digital hybrid-computer
made by anabrid. This is accompanied with a command/executable with the same
name.

# Usage Example

The main purpose of this class is to provide a *thin* layer above the JSONL
protocol (de-)serialization and different endpoints.

	hc, err := lucigo.HybridController("tcp://1.2.3.4")
	if err != nil {
		log.Fatal(err)
	}

	resp, err := hc.Query("net_status")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("net_status = %#v\n", resp)

You can also have a look into the executabe called `lucigo` to get an idea how
to use this library.
*/
package lucigo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/mdns"
	"github.com/tarm/serial"
)

// SendEnvelope is the outer structure of a message sent to LUCIDAC
// in the JSONL protocol.
type SendEnvelope struct {
	Type string      `json:"type"`
	Id   uuid.UUID   `json:"id"`
	Msg  interface{} `json:"msg"`
}

// RecvEnvelope is the outer structure of a received message from LUCIDAC
// in the JSONL protocol. By convention, the Id and Type have to match
// with the previously sent SendEnvelope. The message depends on the Type.
type RecvEnvelope struct {
	Type  string                 `json:"type"`
	Id    uuid.UUID              `json:"id"`
	Code  int                    `json:"code"`
	Error string                 `json:"error"`
	Msg   map[string]interface{} `json:"msg"`
}

// IsSuccess indicates whether the RecvEnvelope contains an Error message
// or not.
func (recv *RecvEnvelope) IsSuccess() bool {
	return recv.Code == 0
}

// NewEnvelope creates a SendEnvelope for a given type with random UUID and emtpy Msg
func NewEnvelope(Type string) SendEnvelope {
	return SendEnvelope{Type: Type, Id: uuid.New()}
}

// LUCIDAC connection endpoints
type Endpoint interface {
	Open() (io.ReadWriter, error)
	ToURL() string

	// Valid checks only for whether the endpoint struct holds useful data,
	// not whether it can actually be opened
	IsValid() bool
}

// TCPEndpoint contains all information neccessary to connect to a TCP/IP
// endpoint. An endpoint is where an actual LUCIDAC serves.
type TCPEndpoint struct {
	Host string
	Port int
}

func (e TCPEndpoint) IsValid() bool {
	return e.Port != 0 && e.Host != ""
}

// Default TCP port for the JSONL protocol
const defaultTcpPort = 5732

func (e *TCPEndpoint) HostPort() string {
	return fmt.Sprintf("%s:%d", e.Host, e.Port) // won't work for IPv6
}

func (e TCPEndpoint) ToURL() string {
	return "tcp:// " + e.HostPort()
}

func (e TCPEndpoint) Open() (io.ReadWriter, error) {
	if !e.IsValid() {
		return nil, fmt.Errorf("Invalid TCP Endpoint (all zero)")
	}
	c, err := net.Dial("tcp", e.HostPort())
	//fmt.Printf("Result is %#v, %#v\n", c, err)
	if err != nil {
		return nil, err
	}
	return c, nil
	//fmt.Printf("Connection is open %#v\n", c)
}

// SerialEndport contains all information neccessary to connect to a local
// USB Serial device.
type SerialEndpoint struct {
	Device string
}

func (e SerialEndpoint) IsValid() bool {
	return e.Device != ""
}

func (e SerialEndpoint) ToURL() string {
	return "serial://" + e.Device
}

func (e SerialEndpoint) Open() (io.ReadWriter, error) {
	c := &serial.Config{Name: e.Device, Baud: 115200}
	sock, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	sock.Flush()
	return sock, nil
}

// ParseEndpoint creates either a JSONLEndpoint or a SerialEndpoint, i.e.
// translates an endpoint URL string to a structure.
func ParseEndpoint(endpoint string) (Endpoint, error) {
	// note that this URL Parsing is far from ideal. But we have unit tests
	// over there in luci_test.go
	u, err := url.Parse(endpoint)
	//fmt.Printf("Parsed URL: %+v %#v\n", u, u)
	if err != nil {
		return nil, fmt.Errorf("could not parse '%s' as Endpoint URL: %+v", endpoint, err)
	}
	if len(u.Host) == 0 || len(u.Scheme) == 0 {
		return nil, fmt.Errorf("need to provide an LUCIDAC Endpoint URL such as tcp://1.2.3.4 or serial://. Given was '%s'", endpoint)
	}

	if u.Scheme == "tcp" {
		strport := u.Port()
		hostname := u.Hostname()
		var port = 0
		if len(strport) == 0 {
			port = defaultTcpPort
		} else {
			port, err = strconv.Atoi(strport)
			if err != nil {
				return nil, fmt.Errorf("expected Port as String, but understood %s as %+v", endpoint, u)
			}
		}
		return TCPEndpoint{hostname, port}, nil
	}

	if u.Scheme == "serial" {
		// at POSIX, serial://foo/bar will be replaced to foo/bar
		if len(u.Host) == 0 && len(u.Path) != 0 {
			return SerialEndpoint{u.Path}, nil
		}
		if len(u.Host) != 0 && len(u.Path) == 0 {
			return SerialEndpoint{u.Host}, nil
		}
		return SerialEndpoint{"/" + u.Host + u.Path}, nil
	}

	return nil, fmt.Errorf("don't know how to understand %v", u)
}

// The HybridController is the most important object in this package and
// provides an OOP interface to the LUCIDAC. This class is sometimes also
// called "LUCIDAC" in other clients.
type HybridController struct {
	Endpoint Endpoint
	Stream   io.ReadWriter // *serial.Port
	Reader   *bufio.Scanner
}

// NewHybridController expects an endpoint URL as string.
// It uses [ParseEndpoint] for translating this to an endpoint structure.
func NewHybridController(endpoint Endpoint) (*HybridController, error) {
	hc := &HybridController{}
	hc.Endpoint = endpoint
	log.Printf("NewHybridController: Connecting to %s ...\n", endpoint)
	var err error
	switch eps := endpoint.(type) {
	case TCPEndpoint:
		hc.Stream, err = eps.Open()
		//fmt.Printf("Connection is open %#v\n", c)
	case SerialEndpoint:
		hc.Stream, err = eps.Open()
	default:
		return nil, fmt.Errorf("NewHybridController doesn't know what to do with %T, %#v", eps, eps)
	}
	if err != nil {
		return nil, err
	}
	hc.Reader = bufio.NewScanner(hc.Stream)

	// Slurp any stuff still there, Serial can be weird
	/*
		fmt.Println("Slurping...")
		for hc.stream.Peek(1) {
			recv_line := hc.reader.Text()
			fmt.Printf("Slurping %s\n", recv_line)
		}
		fmt.Println("Done slurping.")
	*/
	// TODO: check out Peek whcih requires bufio.Reader

	return hc, nil
}

func NewHybridControllerFromString(endpoint string) (*HybridController, error) {
	endpointstruct, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("NewHybridController cannot work with endpoint, %v", err)
	}
	return NewHybridController(endpointstruct)
}

// Command is a low-level command to send and receive envelopes.
// Note how this is a *synchronous* implementation.
func (hc *HybridController) Command(sent_envelope SendEnvelope) (*RecvEnvelope, error) {
	//fmt.Printf("command(%+v)\n", sent_envelope)
	sent_line, err := json.Marshal(sent_envelope)
	if err != nil {
		return nil, err //log.Fatal(err)
	}

	if hc == nil || hc.Stream == nil {
		return nil, fmt.Errorf("cannot write on uninitialized HybridController")
	}

	_, err = hc.Stream.Write(append(sent_line, []byte("\r\n")...))
	if err != nil {
		return nil, err
	}

	var recv_envelope = &RecvEnvelope{}
	for hc.Reader.Scan() {
		recv_line := hc.Reader.Text()
		//fmt.Printf("recv_line=%s\n", recv_line)

		// First test_send_env if it is just an echo
		// Happens typically on the serial line (logging, etc)
		var test_send_env SendEnvelope
		json.Unmarshal([]byte(recv_line), &test_send_env)
		//fmt.Printf("test_send_env=%+v\n", test_send_env)
		//if equals(test_send_env, sent_envelope) {
		if reflect.DeepEqual(test_send_env, sent_envelope) {
			continue
		}

		// We got some real data
		json.Unmarshal([]byte(recv_line), &recv_envelope)
		//fmt.Println(string(s))

		if recv_envelope.Type != sent_envelope.Type {
			fmt.Printf("Warning: Expected %s but got %s", sent_envelope.Type, recv_envelope.Type)
		} // same should be tested with Id

		break
	}
	return recv_envelope, nil
}

// QueryMsg is the high-level command for communicating with the LUCIDAC.
func (hc *HybridController) QueryMsg(Type string, Msg map[string]interface{}) (*RecvEnvelope, error) {
	envelope := NewEnvelope(Type)
	envelope.Msg = Msg
	return hc.Command(envelope)
}

// Query is a high-level command for communicating with the LUCIDAC.
// It is a shorthand for [QueryMsg] sending an *empty* message.
// Some command types (such as `Type="net_status"`) do not expect
// messages.
func (hc *HybridController) Query(Type string) (*RecvEnvelope, error) {
	return hc.Command(NewEnvelope(Type))
}

type Discovery struct {
	entries chan *mdns.ServiceEntry
	found   chan Endpoint
}

func (d *Discovery) checkServer() {
	for entry := range d.entries {
		log.Printf("CheckServer: %v\n", entry)

		resolvableIPv4 := false
		ips, err := net.LookupIP(entry.Host)
		if err != nil {
			//fmt.Printf("Could not resolve Host, take instead %s\n", entry.AddrV4)
		} else {
			for _, ip := range ips {
				if ip.String() == entry.AddrV4.String() {
					resolvableIPv4 = true
				}
			}
		}
		if resolvableIPv4 {
			d.found <- TCPEndpoint{entry.Host, defaultTcpPort}
		} else {
			d.found <- TCPEndpoint{entry.AddrV4.String(), defaultTcpPort}
		}
	}
}

// FindServers currently implements mDNS Zeroconf discovery/detection in the local
// IP broadcast domain.
// TODO: It is supposed to be extended for USB device discovery.
func NewDiscovery() Discovery {
	d := Discovery{
		make(chan *mdns.ServiceEntry, 4),
		make(chan Endpoint),
	}
	go d.checkServer()
	mdns.Lookup("_lucijsonl._tcp", d.entries)
	return d
}

func (d *Discovery) Close() {
	close(d.entries)
	close(d.found)
}

func (d *Discovery) FindAll() []Endpoint {
	var results []Endpoint
	var result Endpoint
	select {
	case result = <-d.found:
		log.Printf("FindAll: Found %v\n", result)
		results = append(results, result)
	case <-time.After(1 * time.Second):
		log.Printf("FindServers: Timed out\n")
	}
	d.Close()
	return results
}

func (d *Discovery) FindMaxOne() (result Endpoint, ok bool) {
	select {
	case result = <-d.found:
		log.Printf("FindMaxOne: Decided for %v\n", result)
		ok = true
	case <-time.After(1 * time.Second):
		log.Printf("FindMaxOne: Timed out\n")
		ok = false
	}
	d.Close()
	return result, ok
}
