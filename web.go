package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

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

func StartWebserver() {
	log.SetFlags(0)

	http.HandleFunc("/", getRoot)

	http.HandleFunc("/ws", ws)

	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
