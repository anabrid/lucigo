package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

var Hc *HybridController

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

func net_set(patch map[string]string) {
	// the incoming patch is flat and uses the following notations:
	//  1) foo.bar = cur[foo][bar]    (one level of nesting)
	//  2) bar     = cur[*][bar]      (shorthands to be searched for)

	curEnv, err := Hc.Query("net_get")
	if err != nil {
		log.Fatal(err)
	}
	cur := curEnv.Msg // current net configuration

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

func main() {
	ctx := kong.Parse(&CLI)
	//fmt.Printf("kong Command: %s\n", ctx.Command())

	if len(CLI.Endpoint.String()) == 0 {
		// TODO: Make discovery before dying
		fmt.Printf("Need to provide a LUCIDAC Endpoint, either with -e or as environment variable LUCIDAC_ENDPOINT\n")
		os.Exit(-1)
	}
	//fmt.Printf("CLI Endpoint: %#v len %d\n", CLI.Endpoint.String(), len(CLI.Endpoint.String()))

	var err error // do not use "Hc :=" because it overwrites global scope Hc
	Hc, err = NewHybridController(CLI.Endpoint.String())

	if err != nil {
		log.Fatal(err)
	}

	switch ctx.Command() {
	case "get <query>":
		//fmt.Printf("Endpoint: %+v\n", CLI.Endpoint)
		res, err := Hc.Query("net_status")
		if err != nil {
			log.Fatal(err)
		}
		jsonPrint(res.Msg)
		//fmt.Printf("%+v\n", res)
	case "detect":
		findServers()
	case "webserver":
		StartWebserver()
	case "net-set <settings>":
		// naming: incoming key/value (from CLI)
		//         outgoing key/value (towards Settings JSON structure)

		net_set(CLI.NetSet.Settings)

		return
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

		proof, err := Hc.QueryMsg("net_set", out)
		if err != nil {
			log.Fatal(err)
		}
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
