# GO_Watcher

Lets you monitor the CPU- and Memory Usage of your System.
Additonally added the number of Goroutines.
Offers a webservice, for remote access, with updates regularly via websockets..
Compiled binary is <10mb in size and has a small memory/cpu footprint.

## Uses
 - Websockets
 - Flags
 - Goolge Charts

## Added Dockerfile for the funsies

Build image with:\
`docker build --tag go_watcher .`

Run container with:\
`docker run --rm -p 8080:8080 go_watcher`


## Input
 - -addr :8080 -> Changes to Port and Address of the Server
 - -period 1s  -> Changes the Update Speed of the Graph/Websocket

## Todo
 - Tests!
 - More Infos
 - Change Chart to Running
 - Add better Timestamp
 - ...