package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Starts the normals server;
// Don´t now if thats the rigth place for it though...
func init() {
	// This starts the default server
	go SetupTestEnv()

	logger.SetLevel(logrus.WarnLevel)
}

func SetupTestEnv() {
	http.HandleFunc("/echo", echo)
	http.HandleFunc("/", home)

	logger.Info("Listening on ", *addr)
	logger.Fatal(http.ListenAndServe(*addr, nil))
}

func Test_home(t *testing.T) {

	assert.HTTPStatusCode(t, home, "GET", "localhost:8080", nil, 200)

	res := assert.HTTPBody(home, "GET", "localhost:8080", nil)
	if res == "" {
		t.Errorf("Requesting a Body from Home failed.")
	}
	// Here should be a test to check for the correct Template.
	// But I dont know how
	// TODO assert.Equal(t, res, homeString)
}

// Test_ConnectionOutput checks for correct Messages from the server.
//
// I think there should be a timeout be implemented into this function,
// else the test might run forever.
func Test_echo(t *testing.T) {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/echo"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	if err != nil {
		// log.Fatal("dial:", err)
		t.Errorf("Could not create the connection, %v", err)
	}
	defer c.Close()

	messageTyp, message, err := c.ReadMessage()
	if err != nil {
		t.Errorf("ReadMessage failed, %v", err)
	}
	assert.Equal(t, messageTyp, 1, "Message is not of Type Text.")

	test_stats := PC_stats{}
	err = json.Unmarshal([]byte(message), &test_stats)
	if err != nil {
		t.Errorf("Unmarshalling failed, %v", err)
	}

	assert.NotNil(t, test_stats.CPU_Load, "CPU Load is nil.")
	assert.NotNil(t, test_stats.Mem_Load, "Mem Load is nil.")
	assert.NotNil(t, test_stats.Goroutines, "Goroutines is nil.")

	assert.GreaterOrEqual(t, test_stats.CPU_Load, 0.0)
	assert.GreaterOrEqual(t, test_stats.Mem_Load, 0.0)
	assert.GreaterOrEqual(t, test_stats.Goroutines, 0)
}

// Tests that maxGos (2500) Connections can be created.
//
// The Test stops after 2 Seconds. If no Errors have
// been generated during that time, the test is OK.
// Tests only run if Server is started manually...
func Test_LotsOfConnections(t *testing.T) {

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
