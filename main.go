package main

import (
	"github.com/syumai/workers-go"
)

func main() {
	gw := buildGateway()
	workers.Serve(gw)
}

