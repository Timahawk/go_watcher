package main

import (
	"context"
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"time"

	_ "net/http/pprof"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
)

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
	// use default options
	upgrader = websocket.Upgrader{}
	// Addr
	addr = flag.String("addr", ":8080", "http service address")
	// PollPeriodFlag
	pollPeriod = flag.Duration("period", 1000*time.Millisecond, "Poll Period in Milliseconds.")

	// Created Stats Struct.
	Stats PC_stats
	// logger
	logger = logrus.New()
)

type PC_stats struct {
	CPU_Load   float64   `json:"cpu_load"`
	Mem_Load   float64   `json:"mem_load"`
	Goroutines int       `json:"goroutines"`
	Timestamp  time.Time `json:"timestamp"`
}

func init() {
	// Starte die Goroutines welche die Daten holen
	go GetCPULoad(*pollPeriod)
	go GetMemLoad(*pollPeriod)
	go GetGoroutines(*pollPeriod)

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logger.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	logger.SetLevel(logrus.DebugLevel)
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

// GetCPUoad changes PC_StatsCPU_Load each interval.
func GetGoroutines(interval time.Duration) {
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ticker.C:
			goes := runtime.NumGoroutine()
			Stats.Goroutines = goes
		}
	}
}

// This funcion is needed to listen on the Closes from the Client side.
//
// If you don´t listen to those messages, the Programm will try to write,
// on a dead connection and fail.
func MessageReceiver(ctx context.Context, conn *websocket.Conn, close chan bool) {
	defer func() {
		logger.Debug("Listen Function exited.")
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	// Technically, this does not need to be a loop.
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			logger.Info("Websocket Connection closed.")

			close <- true
			return
		}
	}
}

// MessageWriter writes to the Connection, at specified interval.
//
// If the connection is closed on the client side, the goroutine is notified
// via the C_close Channel and returns.
// Additionally it regularly writes a PingMessage to the connection.
func MessageWriter(ctx context.Context, conn *websocket.Conn, poll *time.Ticker, pong *time.Ticker) {
	defer func() {
		logger.Debug("Write Function exited.")
	}()

	for {
		select {
		case <-poll.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))

			w, err := conn.NextWriter(websocket.TextMessage)
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

		case <-pong.C:

			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Warn("c.WriteMessage did not work", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// echo implements to Websocket Logik.
// So far I could not come up with an Idea how to stop MessageReceiver.
// Dont know how if thats bad...
func echo(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("Error while upgrading", err)
		return
	}

	logger.Info("WebsocketConnection established by:", r.RemoteAddr)

	// Used to stop MessageWriter
	ctx, cancelfunc := context.WithCancel(r.Context())
	// Channel for PongMessages
	pong := time.NewTicker(pingPeriod)
	// Channel for Polling the CPU/Mem Stats
	poll := time.NewTicker(*pollPeriod)
	// Channel to notify this function if Conn closed on Client Side
	client_close := make(chan bool)

	defer func() {
		cancelfunc()
		pong.Stop()
		poll.Stop()
		conn.Close()
		close(client_close)

	}()

	go MessageReceiver(ctx, conn, client_close)
	go MessageWriter(ctx, conn, poll, pong)

	// Blocking until MessageReceiver gets notified about Closed Connection.
	<-client_close
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
<div id="chart_go"></div>
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

		var go_data;
		var go_chart;
		var go_ws_data;
		var go_index = 0;

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
		
		// create options object with titles, colors, etc.
		let go_options = {
			title: "Num Goroutines",
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
			
			console.log('Message from server ', JSON.parse(event.data));
			ws_data = JSON.parse(event.data)

			data.addRow([index, ws_data.cpu_load]);
			chart.draw(data, options);
			index++;

			mem_data.addRow([mem_index, ws_data.mem_load]);
			mem_chart.draw(mem_data, mem_options);
			mem_index++;

			go_data.addRow([go_index, ws_data.goroutines]);
			go_chart.draw(go_data, go_options);
			go_index++;

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
			// create data object with default value
            go_data = google.visualization.arrayToDataTable([
                ["Year", "Goroutines"],
                [0, 100]
            ]);

			
			chart = new google.visualization.LineChart(
                document.getElementById("chart_div")
            );
            chart.draw(data, options);

			mem_chart = new google.visualization.LineChart(
                document.getElementById("chart_mem")
            );
            mem_chart.draw(mem_data, options);

			go_chart = new google.visualization.LineChart(
                document.getElementById("chart_go")
            );
            go_chart.draw(mem_data, options);
        }
    </script>
</body>
</html>
`))
