package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
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

type HybridController struct {
	stream *serial.Port
	reader *bufio.Scanner
}

func NewHybridController(endpoint string) HybridController {
	hc := HybridController{}
	c := &serial.Config{Name: endpoint, Baud: 115200}
	sock, err := serial.OpenPort(c)
	hc.stream = sock
	hc.stream.Flush()
	if err != nil {
		log.Fatal(err)
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

	return hc
}

func equals(a interface{}, b interface{}) bool {
	ja, aerr := json.Marshal(a)
	jb, berr := json.Marshal(b)

	if aerr != nil {
		fmt.Printf("could not marshal json: %s\n", aerr)
		panic(ja)
	}
	if berr != nil {
		fmt.Printf("could not marshal json: %s\n", aerr)
		panic(jb)
	}

	return string(ja) == string(jb)
}

func jsonPrint(anything map[string]interface{}) {
	//jsonData, err := json.Marshal(anything)
	jsonData, err := json.MarshalIndent(anything, "  ", "    ")

	if err != nil {
		fmt.Printf("could not marshal json: %s\n", err)
		return
	}

	fmt.Printf("%s\n", jsonData)
}

func (hc *HybridController) command(sent_envelope SendEnvelope) RecvEnvelope {
	fmt.Printf("command(%+v)\n", sent_envelope)
	sent_line, err := json.Marshal(sent_envelope)
	if err != nil {
		log.Fatal(err)
	}
	_, err = hc.stream.Write(append(sent_line, []byte("\r\n")...))
	if err != nil {
		log.Fatal(err)
	}

	var recv_envelope RecvEnvelope
	for hc.reader.Scan() {
		recv_line := hc.reader.Text()
		fmt.Printf("recv_line=%s\n", recv_line)

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
	return recv_envelope
}

func (hc *HybridController) queryMsg(Type string, Msg map[string]interface{}) RecvEnvelope {
	envelope := NewEnvelope(Type)
	envelope.Msg = Msg
	return hc.command(envelope)
}

func (hc *HybridController) query(Type string) RecvEnvelope {
	return hc.command(NewEnvelope(Type))
}

func findServers() {
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	go func() {
		for entry := range entriesCh {
			fmt.Printf("Got new entry: %v\n", entry)
		}
	}()

	mdns.Lookup("_lucijsonl._tcp", entriesCh)
	close(entriesCh)
}

var CLI struct {
	Endpoint url.URL `optional:"" short:"e" env:"LUCIDAC_ENDPOINT,LUCIDAC_URL,LUCIDAC"`
	Detect   struct {
	} `cmd:""`
	Webserver struct {
	} `cmd:""`
	Get struct {
		Query string `arg:"" enum:"net,entities,circuit" default:"net"`
	} `cmd:"get" help:"Read out/export information"`
	NetSet struct {
		Settings map[string]string `arg:""`
	} `cmd:"net-set" aliases:"set" help:"Set permanent settings"`
}

func treatBool(val string) any {
	switch strings.ToLower(val) {
	case "true":
		return true
	case "false":
		return false
	default:
		return val
	}
}

var Hc HybridController

func main() {
	Hc = NewHybridController("/dev/ttyACM0")
	ctx := kong.Parse(&CLI)
	fmt.Printf("kong Command: %s\n", ctx.Command())
	switch ctx.Command() {
	case "get <query>":

		Hc.query("net_status")
	case "detect":
		findServers()
	case "webserver":
		StartWebserver()
	case "net-set <settings>":
		// naming: incoming key/value (from CLI)
		//         outgoing key/value (towards Settings JSON structure)
		out := make(map[string]interface{})
		for ink, inv := range CLI.NetSet.Settings {
			inkhead, inktail, ink_is_hierarchical := strings.Cut(ink, ".")
			if ink_is_hierarchical {
				if _, outv_exists := out[inkhead]; !outv_exists {
					out[inkhead] = make(map[string]interface{})
				}
				out[inkhead].(map[string]interface{})[inktail] = treatBool(inv)
			} else {
				out[ink] = treatBool(inv)
			}
		}

		out["no_write"] = true // to test

		fmt.Printf("%+v\n", out)
		jsonPrint(out)

		proof := Hc.queryMsg("net_set", out)
		jsonPrint(proof.Msg)

		// proof to be tested against what is supposed to be like
		// works only easily when first querying with net_get and then
		// just making a deep equal test.

	default:
		fmt.Printf("Unexpected Command: %s\n", ctx.Command())

	}

	/*
	   hc.query("net_get")

	   data := map[string]interface{}{}
	   data["hello"] = []int{1, 2, 3, 4}

	   jsonData, err := json.Marshal(data)

	   	if err != nil {
	   		fmt.Printf("could not marshal json: %s\n", err)
	   		return
	   	}

	   fmt.Printf("json data: %s\n", jsonData)
	*/
}
