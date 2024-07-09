// Copyright (c) 2024 anabrid GmbH
// Contact: https://www.anabrid.com/licensing/
// SPDX-License-Identifier: MIT OR GPL-2.0-or-later

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

//go:embed web-assets/*
var embeddedLucigoAssets embed.FS

var upgrader = websocket.Upgrader{} // use default options

func getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got / request\n")
	io.WriteString(w, "This is my website!\n")
}

func luci2ws(ws *websocket.Conn, done chan struct{}) {
	for Hc.reader.Scan() {
		if err := ws.WriteMessage(websocket.TextMessage, Hc.reader.Bytes()); err != nil {
			ws.Close()
			break
		}
	}

	if Hc.reader.Err() != nil {
		log.Println("scan: ", Hc.reader.Err())
	}

	close(done)
}

func ws(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	done := make(chan struct{})
	go luci2ws(c, done)

	// ws2luci
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)

		_, err = Hc.stream.Write(append(message, []byte("\r\n")...))
		if err != nil {
			log.Println("ws2luci:", err)
			break
		}
	}
}

func webServerIdent(w http.ResponseWriter, r *http.Request) {
	var proxy_target *string
	if Hc != nil && len(Hc.endpoint) != 0 {
		proxy_target = &Hc.endpoint
	}

	ident := map[string]interface{}{
		"webserver": map[string]string{
			"scenario": "proxy",
			"name":     "lucigo",
			"version":  Version,
			"build":    Build,
		},
		"proxy": map[string]string{
			"target": *proxy_target,
		},
		"lucigui": map[string]interface{}{
			"host_static_assets": is_lucigui_bundled(),
			"further_infos_here": nil,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ident)
}

func StartWebserver() {
	log.SetFlags(0)

	log.Printf("Webserver starting at http://0.0.0.0:8000\n")

	matches, err := fs.Glob(embeddedLucigoAssets, "*/*")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Embedded files: %+v\n", matches)

	http.HandleFunc("/", getRoot) // also any 404...
	http.HandleFunc("/.well-known/lucidac.json", webServerIdent)
	http.HandleFunc("/ws", ws)

	// serve build-time embedded snapshot of directory
	http.Handle("/embedded/", http.StripPrefix("/embedded/", http.FileServer(http.FS(embeddedLucigoAssets))))

	// serve directory live, can be changed at runtime
	// interestingly, this is without the prefix.
	http.Handle("/local/", http.StripPrefix("/local/", http.FileServer(http.Dir("web-assets"))))

	err = http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
