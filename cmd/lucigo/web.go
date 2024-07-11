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

	"github.com/anabrid/lucigo"
	"github.com/gorilla/websocket"
)

//go:embed web-assets/*
var embeddedLucigoAssets embed.FS

type LuciGoWebServer struct {
	// should also store other options
	hc       *lucigo.HybridController
	upgrader websocket.Upgrader
}

func (server *LuciGoWebServer) getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got / request\n")
	io.WriteString(w, "This is my website!\n")
}

func (server *LuciGoWebServer) luci2ws(ws *websocket.Conn, done chan struct{}) {
	for server.hc.Reader.Scan() {
		if err := ws.WriteMessage(websocket.TextMessage, server.hc.Reader.Bytes()); err != nil {
			ws.Close()
			break
		}
	}

	if server.hc.Reader.Err() != nil {
		log.Println("scan: ", server.hc.Reader.Err())
	}

	close(done)
}

func (server *LuciGoWebServer) startWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	done := make(chan struct{})
	go server.luci2ws(c, done)

	// ws2luci
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)

		_, err = server.hc.Stream.Write(append(message, []byte("\r\n")...))
		if err != nil {
			log.Println("ws2luci:", err)
			break
		}
	}
}

func (server *LuciGoWebServer) webServerIdent(w http.ResponseWriter, r *http.Request) {
	var proxy_target *string
	if server.hc != nil && len(server.hc.Endpoint) != 0 {
		proxy_target = &server.hc.Endpoint
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

func (server *LuciGoWebServer) StartWebserver() {
	log.SetFlags(0)

	log.Printf("Webserver starting at http://0.0.0.0:8000\n")

	matches, err := fs.Glob(embeddedLucigoAssets, "*/*")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Embedded files: %+v\n", matches)

	http.HandleFunc("/", server.getRoot) // also any 404...
	http.HandleFunc("/.well-known/lucidac.json", server.webServerIdent)
	http.HandleFunc("/ws", server.startWebSocket)

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

func NewLuciGoWebServer(hc *lucigo.HybridController) (server *LuciGoWebServer) {
	return &LuciGoWebServer{hc: hc, upgrader: websocket.Upgrader{ /*defaults*/ }}
}
