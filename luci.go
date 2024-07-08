package main

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

type SendEnvelope struct {
	Type string      `json:"type"`
	Id   uuid.UUID   `json:"id"`
	Msg  interface{} `json:"msg"`
}

type RecvEnvelope struct {
	Type string                 `json:"type"`
	Id   uuid.UUID              `json:"id"`
	Code int                    `json:"code"`
	Msg  map[string]interface{} `json:"msg"`
}

func NewEnvelope(Type string) SendEnvelope {
	// Simplifying, nectlecting msg
	return SendEnvelope{Type: Type, Id: uuid.New()}
}

//type Endpoint string

type JSONLEndpoint struct {
	Host     string
	Port     int
	HostPort string
}

type SerialEndpoint struct {
	Device string
}

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
		return JSONLEndpoint{hostname, port, u.Host}, nil
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

type HybridController struct {
	endpoint      string
	endpoint_type interface{}
	stream        io.ReadWriter // *serial.Port
	reader        *bufio.Scanner
}

func NewHybridController(endpoint string) (*HybridController, error) {
	endpointstruct, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("NewHybridController cannot work with endpoint, %v", err)
	}
	hc := &HybridController{}
	hc.endpoint = endpoint
	hc.endpoint_type = endpointstruct
	switch eps := endpointstruct.(type) {
	case JSONLEndpoint:
		//fmt.Printf("Dialing... %#v\n", eps)
		c, err := net.Dial("tcp", eps.HostPort)
		//fmt.Printf("Result is %#v, %#v\n", c, err)
		if err != nil {
			log.Fatal(err)
		}
		hc.stream = c
		//fmt.Printf("Connection is open %#v\n", c)
	case SerialEndpoint:
		c := &serial.Config{Name: eps.Device, Baud: 115200}
		sock, err := serial.OpenPort(c)
		sock.Flush()
		hc.stream = sock
		if err != nil {
			log.Fatal(err)
		}
	default:
		return nil, fmt.Errorf("NewHybridController doesn't know what to do with %T, %#v", eps, eps)
	}
	hc.reader = bufio.NewScanner(hc.stream)

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

func (hc *HybridController) command(sent_envelope SendEnvelope) (*RecvEnvelope, error) {
	//fmt.Printf("command(%+v)\n", sent_envelope)
	sent_line, err := json.Marshal(sent_envelope)
	if err != nil {
		return nil, err //log.Fatal(err)
	}

	if hc.stream == nil {
		return nil, fmt.Errorf("Cannot write on uninitialized HybridController")
	}

	_, err = hc.stream.Write(append(sent_line, []byte("\r\n")...))
	if err != nil {
		return nil, err
	}

	var recv_envelope = &RecvEnvelope{}
	for hc.reader.Scan() {
		recv_line := hc.reader.Text()
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

func (hc *HybridController) queryMsg(Type string, Msg map[string]interface{}) (*RecvEnvelope, error) {
	envelope := NewEnvelope(Type)
	envelope.Msg = Msg
	return hc.command(envelope)
}

func (hc *HybridController) query(Type string) (*RecvEnvelope, error) {
	return hc.command(NewEnvelope(Type))
}

func findServers() {
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
