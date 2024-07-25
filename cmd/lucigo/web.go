// Copyright (c) 2024 anabrid GmbH
// Contact: https://www.anabrid.com/licensing/
// SPDX-License-Identifier: MIT OR GPL-2.0-or-later

package main

import (
	"archive/zip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/anabrid/lucigo"
	"github.com/gorilla/websocket"
)

//go:embed web-assets/*
var embeddedLucigoAssets embed.FS

// access is_lucigui_bundled() to check whether this is populated, given by build system.

type LuciGoWebServer struct {
	// should also store other options
	ListenAddress  string
	Hc             *lucigo.HybridController
	Upgrader       websocket.Upgrader
	AllowOrigin    string
	StaticPath     string
	primaryGUIpath string // set internally at construction
}

func (server *LuciGoWebServer) getRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, server.primaryGUIpath, http.StatusTemporaryRedirect)
	io.WriteString(w, "GUI is served at "+server.primaryGUIpath+"\n")
}

func (server *LuciGoWebServer) luci2ws(ws *websocket.Conn, done chan struct{}) {
	for server.Hc.Reader.Scan() {
		if err := ws.WriteMessage(websocket.TextMessage, server.Hc.Reader.Bytes()); err != nil {
			ws.Close()
			break
		}
	}

	if server.Hc.Reader.Err() != nil {
		log.Println("scan: ", server.Hc.Reader.Err())
	}

	close(done)
}

func (server *LuciGoWebServer) startWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := server.Upgrader.Upgrade(w, r, nil)
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

		_, err = server.Hc.Stream.Write(append(message, []byte("\r\n")...))
		if err != nil {
			log.Println("ws2luci:", err)
			break
		}
	}
}

func (server *LuciGoWebServer) webServerIdent(w http.ResponseWriter, r *http.Request) {
	var proxy_target *string
	if server.Hc != nil && len(server.Hc.Endpoint) != 0 {
		proxy_target = &server.Hc.Endpoint
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

func (server *LuciGoWebServer) DaemonRun() (server_err chan error) {
	server_err = make(chan error)
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
	return server_err
}

func DaemonWait(server_err chan error) {
	// wait until webserver completed
	err_val := <-server_err
	if err_val != nil {
		log.Fatal(err_val)
	}
}

// Note that this function starts the server in sync. Use a goroutine
// around it if you want to start it in background
func (server *LuciGoWebServer) StartWebserver() error {
	log.SetFlags(0)

	log.Printf("Webserver starting at http://0.0.0.0:8000\n")

	// this is how to also print what is embedded at build time:
	matches, err := fs.Glob(embeddedLucigoAssets, "*/*")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("StartWebserver: Embedded files: %+v\n", matches)

	http.HandleFunc("/", server.getRoot) // also any 404...
	http.HandleFunc("/.well-known/lucidac.json", server.webServerIdent)
	http.HandleFunc("/ws", server.startWebSocket)

	// serve build-time embedded snapshot of directory
	if is_lucigui_bundled() {
		http.Handle("/embedded/", http.StripPrefix("/embedded/", http.FileServer(http.FS(embeddedLucigoAssets))))
		// TODO check if path exists
		server.primaryGUIpath = "/embedded/lucigui"
	}

	if server.StaticPath != "" {
		fileInfo, err := os.Stat(server.StaticPath)
		var fs http.FileSystem = nil
		if err != nil {
			log.Printf("registerLocalFiles: ERROR, path %s not readable: %v\n", server.StaticPath, err)
		} else {
			if fileInfo.IsDir() {
				log.Printf("registerLocalFiles: serving %s at /local\n", server.StaticPath)
				fs = http.Dir(server.StaticPath)
			} else if strings.ToLower(filepath.Ext(server.StaticPath)) == ".zip" {
				fh, ziperr := zip.OpenReader(server.StaticPath)
				if ziperr != nil {
					log.Printf("registerLocalFiles: Cannot open ZIP file %s, reason: %s", server.StaticPath, ziperr)
				} else {
					log.Printf("registerLocalFiles: serving ZIP file %s at /local\n", server.StaticPath)
					fs = http.FS(fh)
				}
			} else {
				log.Printf("registerLocalFiles: ERROR, path %s is neither directory nor .zip file!\n", server.StaticPath)
			}
			if fs != nil {
				http.Handle("/local/", http.StripPrefix("/local/", http.FileServer(fs)))
				// TODO check if path exists
				server.primaryGUIpath = "/local/lucigui"
			}
		}
	}

	err = http.ListenAndServe(server.ListenAddress, nil)
	return err
}

func NewLuciGoWebServer(hc *lucigo.HybridController) (server *LuciGoWebServer) {
	return &LuciGoWebServer{
		Hc:             hc,
		ListenAddress:  "127.0.0.1:8000",
		primaryGUIpath: "/index.html",
		Upgrader:       websocket.Upgrader{ /*defaults*/ },
	}
}
