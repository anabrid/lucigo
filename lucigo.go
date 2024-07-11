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

// isSuccess indicates whether the RecvEnvelope contains an Error message
// or not.
func RecvIsSuccess(recv *RecvEnvelope) bool {
	return recv.Code == 0
}

// NewEnvelope creates a SendEnvelope for a given type with random UUID and emtpy Msg
func NewEnvelope(Type string) SendEnvelope {
	return SendEnvelope{Type: Type, Id: uuid.New()}
}

// JSONLEndpoint contains all information neccessary to connect to a TCP/IP
// endpoint. An endpoint is where an actual LUCIDAC serves.
type JSONLEndpoint struct {
	Host string
	Port int
}

func (e *JSONLEndpoint) HostPort() string {
	return fmt.Sprintf("%s:%d", e.Host, e.Port) // won't work for IPv6
}

// SerialEndport contains all information neccessary to connect to a local
// USB Serial device.
type SerialEndpoint struct {
	Device string
}

// ParseEndpoint creates either a JSONLEndpoint or a SerialEndpoint, i.e.
// translates an endpoint URL string to a structure.
func ParseEndpoint(endpoint string) (interface{}, error) {
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
		if len(strport) == 0 {
			strport = "5732" // default port
		}
		port, err := strconv.Atoi(strport)
		if err != nil {
			return nil, fmt.Errorf("expected Port as String, but understood %s as %+v", endpoint, u)
		}
		return JSONLEndpoint{hostname, port}, nil
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
	Endpoint      string
	Endpoint_type interface{}
	Stream        io.ReadWriter // *serial.Port
	Reader        *bufio.Scanner
}

// NewHybridController expects an endpoint URL as string.
// It uses [ParseEndpoint] for translating this to an endpoint structure.
func NewHybridController(endpoint string) (*HybridController, error) {
	endpointstruct, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("NewHybridController cannot work with endpoint, %v", err)
	}
	hc := &HybridController{}
	hc.Endpoint = endpoint
	hc.Endpoint_type = endpointstruct
	switch eps := endpointstruct.(type) {
	case JSONLEndpoint:
		//fmt.Printf("Dialing... %#v\n", eps)
		c, err := net.Dial("tcp", eps.HostPort())
		//fmt.Printf("Result is %#v, %#v\n", c, err)
		if err != nil {
			log.Fatal(err)
		}
		hc.Stream = c
		//fmt.Printf("Connection is open %#v\n", c)
	case SerialEndpoint:
		c := &serial.Config{Name: eps.Device, Baud: 115200}
		sock, err := serial.OpenPort(c)
		sock.Flush()
		hc.Stream = sock
		if err != nil {
			log.Fatal(err)
		}
	default:
		return nil, fmt.Errorf("NewHybridController doesn't know what to do with %T, %#v", eps, eps)
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

// FindServers currently implements mDNS Zeroconf discovery in the local
// IP broadcast domain.
// TODO: It is supposed to be extended for USB device discovery.
func FindServers() {
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	go func() {
		for entry := range entriesCh {
			// TODO, check with https://pkg.go.dev/net#LookupHost
			// whether we can resolve that entry, similar to the python
			// version.
			// also make sure we collect the data back at the main function,
			// i.e. return what is in entriesCh
			fmt.Printf("Got new entry: %v\n", entry)
		}
	}()

	mdns.Lookup("_lucijsonl._tcp", entriesCh)
	close(entriesCh)
}
