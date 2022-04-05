package main

import (
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func SetupTestEnv() {
	http.HandleFunc("/echo", echo)
	http.HandleFunc("/", home)

	logger.Info("Listening on ", *addr)
	logger.Fatal(http.ListenAndServe(*addr, nil))
}

// Tests that maxGos (2500) Connections can be created.
//
// The Test stops after 2 Seconds. If no Errors have
// been generated during that time, the test is OK.
// Tests only run if Server is started manually...
func Test_LotsOfConnections(t *testing.T) {

	go SetupTestEnv()

	done := make(chan bool)
	maxGos := 2500

	fails := make(chan int)

	timer := time.NewTimer(2 * time.Second)

	for i := 0; i < maxGos; i++ {
		go CreateConnection(i, fails)
	}
	select {
	case <-done:
		return
	case err := <-fails:
		t.Fatalf("Jep sth went wrong:, %v", err)
		return
	case <-timer.C:
		return

	}
}

func CreateConnection(i int, fails chan int) {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/echo"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		// log.Fatal("dial:", err)
		fails <- i
		return
	}
	if i > 2500 {
		log.Println("Connected to", u.String(), "Connection number:", i)
	}
	defer c.Close()

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			fails <- i
			return
		}
		// log.Printf("recv: %s", message)
	}
}
