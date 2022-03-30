package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var upgrader = websocket.Upgrader{} // use default options

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

	Stats PC_stats
	// Wie oft die Website geupdated wird.
	//pollPeriod = time.Millisecond * 1000
)

type PC_stats struct {
	CPU_Load float64 `json:"cpu_load"`
	Mem_Load float64 `json:"mem_load"`
}

func init() {
	// Starte die Goroutines welche die Daten holen
	go GetCPULoad(*pollPeriod)
	go GetMemLoad(*pollPeriod)
}

func main() {
	flag.Parse()

	http.HandleFunc("/echo", echo)
	http.HandleFunc("/", home)

	log.Println("Listening on X", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func GetMemLoad(interval time.Duration) {
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ticker.C:
			v, _ := mem.VirtualMemory()
			Stats.Mem_Load = v.UsedPercent
		}
	}
}

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

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error while upgrading", err)
		return
	}

	ticker := time.NewTicker(pingPeriod)
	poll := time.NewTicker(*pollPeriod)
	defer func() {
		ticker.Stop()
		poll.Stop()
		c.Close()
	}()
	for {
		select {
		case <-poll.C:
			c.SetWriteDeadline(time.Now().Add(writeWait))

			w, err := c.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Fatalln("c.NextWriter did not work", err)
			}
			json, err := json.Marshal(Stats)
			if err != nil {
				log.Fatalln("Marshalling did not work", err)
			}

			w.Write(json)

			if err := w.Close(); err != nil {
				log.Println("io.Writer Close did not work", err, r)
			}
		case <-ticker.C:

			c.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Fatalln("c.WriteMessage did not work", err)
			}
		}
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	log.Println("Connection", r)
	homeTemplate.Execute(w, nil)
}

// https://stackoverflow.com/questions/43693360/convert-float64-to-byte-array
func float64ToByte(f float64) []byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(f))
	return buf[:]
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
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
