package main

import (
	"flag"

	"driver-plugin/pkg/devicescheduler"

	"github.com/wh8199/log"
)

var (
	edgeServer string
	edgePort   int
	token      string
)

func init() {
	flag.StringVar(&edgeServer, "edge-server", "", "the server ip of edge")
	flag.IntVar(&edgePort, "edge-server-port", 8090, "the server port of edge")
	flag.StringVar(&token, "token", "", "driver plugin access token")

	flag.Parse()
}

func main() {
	s, err := devicescheduler.NewDeviceScheduler(edgeServer, token, edgePort)
	if err != nil {
		log.Fatal(err)
	}

	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
	log.Info("device scheduler starts successfully")
}
