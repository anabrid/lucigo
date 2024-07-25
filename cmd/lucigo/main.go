// Copyright (c) 2024 anabrid GmbH
// Contact: https://www.anabrid.com/licensing/
// SPDX-License-Identifier: MIT OR GPL-2.0-or-later

/*
lucigo is a client for the LUCIDAC and a reference implementation for the
similiarly named golang package which allows to write clients in the go
programming language. The executable has a focus on device administration
and is not a general-purpose client, i.e. it does not expose support for
simplifying analog circuit configuration. In contrast, the tool provides
support for networking with the LUCIDAC. For instance, it allows to easily
bring a USB device into the network ("proxying") and can also run a webserver
to host the [lucigui](https://lucidac.online/). *lucigui* is the web-based
LUCIDAC client written in Svelte/TypeScript and not to be confused with
*lucigo*.

Given the static nature of go builds, it is also an excellent tool for
getting started with LUCIDAC, given that no
dependencies have to be installed and users do not have to opt-in to some
ecosystem. This way, lucigo may also be a useful tool for shell scripting
a LUCIDAC.
*/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/anabrid/lucigo"
	"github.com/nqd/flat"
)

var (
	Hc *lucigo.HybridController // used as a global also in web.go

	// these variables to be set with
	//   go run -ldflags "-X lucigo.version=1.2.3 build_shorthash=c3e7fe1 lucigui_bundled=true"
	// TODO, implement in a makefile, cf. for instance
	// https://stackoverflow.com/questions/11354518/application-auto-build-versioning#11355611
	Version         string
	Build           string
	lucigui_bundled string
)

func is_lucigui_bundled() bool {
	return strings.ToLower(lucigui_bundled) == "true"
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

func net_get() {
	// TODO: Does not handle the following values well:
	//       Null, empty lists/maps, empty strings
	res, err := getHybridController().Query("net_get")
	if err != nil {
		log.Fatal(err)
	}
	if !res.IsSuccess() {
		log.Fatalf("net_get returned code %d: %s", res.Code, res.Error)
	}
	flattened_settings, err := flat.Flatten(res.Msg, nil)
	if err != nil {
		log.Fatalf("Flattening of net_get failed: %s\n", err)
	}
	keys := keys(flattened_settings)
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Print(k)
		fmt.Print(" = ")
		fmt.Println(flattened_settings[k])
	}
}

func net_set(patch map[string]string) {
	// the incoming patch is flat and uses the following notations:
	//  1) foo.bar = cur[foo][bar]    (one level of nesting)
	//  2) bar     = cur[*][bar]      (shorthands to be searched for)

	curEnv, err := getHybridController().Query("net_get")
	if err != nil {
		log.Fatal(err)
	}
	cur := curEnv.Msg // current net configuration

	// TODO: use flat.Unflatten / flat.Flatten as in net-set!

	// TODO Deep-copy cur

	// search for the data
	for ink, inv := range CLI.NetSet.Settings {
		inkhead, inktail, ink_is_hierarchical := strings.Cut(ink, ".")

		if !ink_is_hierarchical {
			// search for the key in all dictionaries (notation 2)
			cand_head := []string{}
			for park, parv := range cur {
				pard, pard_is_dict := parv.(map[string]interface{})
				if !pard_is_dict {
					continue // cannot descend into scalar
				} else if _, node_exists := pard[ink]; node_exists {
					// have found a node candidate where the key belongs to!
					cand_head = append(cand_head, park)
				}
			}

			if len(cand_head) == 0 {
				// no candidate found -> make it tomost
				cur[ink] = treatBool(inv)
			} else if len(cand_head) == 1 {
				// exactly one candidate found -> shorthand worked
				cur[cand_head[0]].(map[string]interface{})[ink] = treatBool(inv)
			} else {
				// multiple candidates found. Make it an error.
				// Also improve error message.
				fmt.Printf("Found multiple candidates for key '%s', specify it fully qualified with 'some-prefix.%s' %+v", ink, ink, cand_head)
			}
		} else { // if ink_is_hierarchical {
			if _, curv_exists := cur[inkhead]; !curv_exists {
				// have to create the parent node
				cur[inkhead] = make(map[string]interface{})
			}
			// if the datum exists already, could also adopt for the data type
			// and raise an error if it doesn't match at all
			cur[inkhead].(map[string]interface{})[inktail] = treatBool(inv)
		}
	}

	jsonPrint(cur)
}

type versionFlag bool
type verboseFlag bool

func (d versionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	if len(Version) == 0 {
		Version = "(no version information)"
	}
	if len(Build) == 0 {
		Build = "(no build information)"
	}
	fmt.Printf("lucigo/%s build %s (lucigui bundled: %v)\n", Version, Build, is_lucigui_bundled())
	app.Exit(0)
	return nil
}

func keys(m map[string]interface{}) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func tryFindServers() (endpoint string) {
	endpoint = lucigo.FindServers()
	if len(endpoint) == 0 {
		fmt.Fprintf(os.Stderr, "No Endpoint found (tried Zeroconf). Provide a LUCIDAC Endpoint, either with -e or as environment variable LUCIDAC_ENDPOINT\n")
		os.Exit(1)
	}
	return endpoint
}

func cliOrTryFindServers() (endpoint string) {
	endpoint = CLI.Endpoint.String()
	if len(endpoint) == 0 {
		return tryFindServers()
	}
	return endpoint
}

func getHybridController() *lucigo.HybridController {
	endpoint := cliOrTryFindServers()
	Hc, err := lucigo.NewHybridController(endpoint)
	if err != nil {
		log.Fatal(err)
		os.Exit(2)
	}
	return Hc
}

func isReachable(address string) bool {
	timeout := 500 * time.Millisecond
	log.Printf("isReachable: Testing %s for a time %v\n", address, timeout)
	_, err := net.DialTimeout("tcp", address, timeout)
	return err == nil
}

func isURLReachable(url string) bool {
	client := http.Client{
		Timeout: 800 * time.Millisecond,
	}
	recv, err := client.Get(url)
	return err == nil && recv.StatusCode < 400
}

func openWebBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		log.Printf("openWebBrowser: Calling xdg-open %s\n", url)
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		log.Printf("openWebBrowser: Calling rundll32 url.dll,FileProtocolHandler %s\n", url)
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		log.Printf("openWebBrowser: Calling open %s\n", url)
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("please point your browser to this URL: %s", url)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func Start() {
	endpoint := cliOrTryFindServers()
	Hc, err := lucigo.NewHybridController(endpoint)
	if err != nil {
		log.Fatal(err)
	}

	canUseEmbeddedWebserver := false
	targetUrl := ""

	switch endpoint := Hc.Endpoint_type.(type) {
	case lucigo.TCPEndpoint:
		// checks both for available server and if LUCIGUI is embedded in firmware
		candidateUrl := "http://" + endpoint.Host + "/lucigui/"
		log.Printf("Start: Testing whether %s is reachable\n", candidateUrl)
		canUseEmbeddedWebserver = isURLReachable(candidateUrl)
		if canUseEmbeddedWebserver {
			targetUrl = candidateUrl
		}
	case lucigo.SerialEndpoint:
		canUseEmbeddedWebserver = false
	}

	server_err := make(chan error)
	if canUseEmbeddedWebserver {
		log.Printf("Start: Can reach embedded Webserver at %s\n", targetUrl)
	} else {
		server := NewLuciGoWebServer(Hc)
		targetUrl = "http://" + server.listenAddress
		log.Printf("Start: Cannot reach embedded Webserver. Launching webserver at %s\n", targetUrl)
		go func() {
			server_err <- server.StartWebserver()
			log.Println("Server already finished")
		}()
		select {
		case err_val, received := <-server_err:
			if received {
				log.Fatalln("Webserver prematurly ended.")
				if err_val != nil {
					log.Fatal(server_err)
				}
			}
		default: // no error received, still running
		}
	}

	openWebBrowser(targetUrl)

	if !canUseEmbeddedWebserver {
		// wait until webserver completed
		err_val := <-server_err
		if err_val != nil {
			log.Fatal(err_val)
		}
	}
}

var CLI struct {
	Endpoint url.URL     `optional:"" short:"e" env:"LUCIDAC_ENDPOINT,LUCIDAC_URL,LUCIDAC" help="The lucidac to connect to"`
	Version  versionFlag `optional:"" help:"Show version information (only, then exit)"`
	Verbose  verboseFlag `optional:"" short:"v" help:"Get more verbose output"`
	Detect   struct {
	} `cmd:"" help:"Detect any LUCIDAC, print and exit"`
	Start struct {
	} `cmd:"" help:"Getting started quickly - Open any appropriate GUI in webbrowser. Runs per default if no argument is given"`
	Webserver struct {
	} `cmd:"" help:"Launch internal webserver with GUI. Won't fall back to embedded webserver."`
	Query struct {
		Type string `arg:"" optional:"" default:"help"`
	} `cmd:"query" help:"Ask a raw query without arguments"`
	NetGet struct {
	} `cmd:"net-get" help:"Read out permanent settings"`
	NetSet struct {
		Settings map[string]string `arg:""`
	} `cmd:"net-set" aliases:"set" help:"Set permanent settings"`
}

func main() {
	if len(os.Args) == 1 {
		// Kong does not accept default commands. For no arguments given,
		// short-circuit Kong and instead call Start().
		// note that there is no way to decrease verbosity here.
		Start()
		return
	}

	desc := kong.Description("LUCIGO is an administrative client for the LUCIDAC analog digital hybrid computer. It provides a command line interface for simplifying the device lookup and administration. It furthermore provides built in proxy services and can start up the web-based GUI on an USB-connected LUCIDAC. Consider the README for more information at https://github.com/anabrid/lucigo")
	ctx := kong.Parse(&CLI, desc, kong.UsageOnError())
	//fmt.Printf("kong Command: %s, %+v\n", ctx.Command(), CLI)

	if !CLI.Verbose {
		log.SetOutput(io.Discard)
	}

	switch ctx.Command() {
	case "query <type>":
		res, err := getHybridController().Query(CLI.Query.Type)
		if err != nil {
			log.Fatal(err)
		}
		jsonPrint(res.Msg)
		//fmt.Printf("%+v\n", res)
	case "detect":
		endpoint := tryFindServers()
		fmt.Println(endpoint)
	case "webserver":
		Hc := getHybridController()
		NewLuciGoWebServer(Hc).StartWebserver()
	case "net-get":
		net_get()
	case "net-set <settings>":
		// naming: incoming key/value (from CLI)
		//         outgoing key/value (towards Settings JSON structure)
		net_set(CLI.NetSet.Settings)
		return
	default:
		fmt.Printf("Unexpected Command: %s\n", ctx.Command())
	}
}
