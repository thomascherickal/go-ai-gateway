package main

import (
	"github.com/syumai/workers-go"
)

var serveHandler = workers.Serve

func main() {
	gw := buildGateway()
	serveHandler(gw)
}
