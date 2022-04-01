package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"os"
	"time"

	_ "net/http/pprof"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
)

// use default options
var upgrader = websocket.Upgrader{}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	// Addr
	addr = flag.String("addr", ":8080", "http service address")
	// PollPeriodFlag
	pollPeriod = flag.Duration("period", 1000*time.Millisecond, "Poll Period in Milliseconds.")

	// Created Stats Struct.
	Stats PC_stats

	logger = logrus.New()
)

type PC_stats struct {
	CPU_Load  float64   `json:"cpu_load"`
	Mem_Load  float64   `json:"mem_load"`
	Timestamp time.Time `json:"timestamp"`
}

func init() {
	// Starte die Goroutines welche die Daten holen
	go GetCPULoad(*pollPeriod)
	go GetMemLoad(*pollPeriod)

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logger.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	logger.SetLevel(logrus.InfoLevel)
	//
	logger.SetFormatter(
		&logrus.TextFormatter{TimestampFormat: "2006/01/02 - 15:04:05",
			FullTimestamp: true})
}

func main() {

	flag.Parse()

	http.HandleFunc("/echo", echo)
	http.HandleFunc("/", home)

	logger.Info("Listening on ", *addr)
	logger.Fatal(http.ListenAndServe(*addr, nil))

}

// GetMemLoad changes PC_Stats.Mem_Load each interval.
//
// This goroutine also writes the Timestamp to PC_Stats.
func GetMemLoad(interval time.Duration) {
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ticker.C:
			v, err := mem.VirtualMemory()
			if err != nil {
				panic(err)
			}
			Stats.Mem_Load = v.UsedPercent
			Stats.Timestamp = time.Now()
		}
	}
}

// GetCPUoad changes PC_StatsCPU_Load each interval.
func GetCPULoad(interval time.Duration) {
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ticker.C:
			load, err := cpu.Percent(time.Second*0, false)
			if err != nil {
				panic(err)
			}
			Stats.CPU_Load = load[0]
		}
	}
}

// This funcion is needed to listen on the Closes from the Client side.
//
// If you don´t listen to those messages, the Programm will try to write,
// on a dead connection and fail.
func listen_function(c *websocket.Conn, message chan []byte, C_close chan bool) {
	c.SetReadLimit(maxMessageSize)
	c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error { c.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, mess, err := c.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Info("IsUnexpectedCloseError", err)
			}

			logger.Info("Websocket Connection closed.")

			message <- mess
			close(message)
			C_close <- true
			close(C_close)

			return
		}
	}
}

// write_function writes to the Connection, at specified interval.
//
// If the connection is closed on the client side, the goroutine is notified
// via the C_close Channel and returns.
// Additionally it regularly writes a PingMessage to the connection.
func write_function(c *websocket.Conn, poll *time.Ticker, ticker *time.Ticker, C_close chan bool) {
	for {
		select {
		case <-poll.C:
			c.SetWriteDeadline(time.Now().Add(writeWait))

			w, err := c.NextWriter(websocket.TextMessage)
			if err != nil {
				logger.Warn("c.NextWriter did not work", err)
			}
			json, err := json.Marshal(Stats)
			if err != nil {
				logger.Fatalln("Marshalling did not work", err)
			}

			w.Write(json)

			if err := w.Close(); err != nil {
				logger.Warn("io.Writer Close did not work", err)
			}
		case <-ticker.C:

			c.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Warn("c.WriteMessage did not work", err)
			}
		case <-C_close:
			return
		}
	}
}

// Websocket Connection handler.
//
// TODO implement a way to stop the listen_function when write_functions gets an error.
func echo(w http.ResponseWriter, r *http.Request) {

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("Error while upgrading", err)
		return
	}

	logger.Info("WebsocketConnection established by:", r.RemoteAddr)

	ticker := time.NewTicker(pingPeriod)
	poll := time.NewTicker(*pollPeriod)

	message := make(chan []byte)

	C_close := make(chan bool)

	defer func() {
		ticker.Stop()
		poll.Stop()
		c.Close()

	}()

	go listen_function(c, message, C_close)
	go write_function(c, poll, ticker, C_close)

	<-message
}

func home(w http.ResponseWriter, r *http.Request) {

	// Das hier läuft zweimail weil, /favicon.ico auch hierher geroutete wird.
	logger.Info("Connection", r.RemoteAddr)
	homeTemplate.Execute(w, nil)
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<link rel="icon" href="data:,">
</head>
<body>
<div id="chart_div"></div>
<div id="chart_mem"></div>
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
    <script>
		var data;
		var chart;
		var ws_data;
		var index = 0;

		var mem_data;
		var mem_chart;
		var mem_ws_data;
		var mem_index = 0;

		// create options object with titles, colors, etc.
		let options = {
			title: "CPU Usage",
			hAxis: {
				title: "Time"
			},
			vAxis: {
				title: "Usage"
			}
		};

		// create options object with titles, colors, etc.
		let mem_options = {
			title: "Memory Usage",
			hAxis: {
				title: "Time"
			},
			vAxis: {
				title: "Usage"
			}
		};
		
		ws = new WebSocket("ws://" + document.location.host + "/echo");

		ws.onopen = function(evt) {
			console.log("OPEN");
		}

		ws.onclose = function(evt) {
			document.getElementById("Load").innerText = "Closed by Server"
			console.log("CLOSE");
		}

		// Listen for messages
		ws.addEventListener('message', function (event) {
			// document.getElementById("Load").innerText = event.data
			console.log('Message from server ', JSON.parse(event.data));
			ws_data = JSON.parse(event.data)
			data.addRow([index, ws_data.cpu_load]);
			chart.draw(data, options);
			index++;

			mem_data.addRow([mem_index, ws_data.mem_load]);
			mem_chart.draw(mem_data, mem_options);
			mem_index++;

		});

		ws.onerror = function(evt) {
			document.getElementById("Load").innerText = "Erro occured"
			console.log("ERROR: " + evt);
		}

        // load current chart package
        google.charts.load("current", {
            packages: ["corechart", "line"]
        });
        // set callback function when api loaded
        google.charts.setOnLoadCallback(drawChart);
        function drawChart() {
            // create data object with default value
            data = google.visualization.arrayToDataTable([
                ["Year", "CPU Usage"],
                [0, 0]
            ]);
			// create data object with default value
            mem_data = google.visualization.arrayToDataTable([
                ["Year", "Mem Usage"],
                [0, 100]
            ]);

            // draw chart on load
            chart = new google.visualization.LineChart(
                document.getElementById("chart_div")
            );
            chart.draw(data, options);

			chart = new google.visualization.LineChart(
                document.getElementById("chart_div")
            );
            chart.draw(data, options);

			// draw chart on load
            mem_chart = new google.visualization.LineChart(
                document.getElementById("chart_mem")
            );
            mem_chart.draw(mem_data, options);

			mem_chart = new google.visualization.LineChart(
                document.getElementById("chart_mem")
            );
            mem_chart.draw(mem_data, options);
        }
    </script>
</body>
</html>
`))
