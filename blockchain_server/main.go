package main

import (
	"flag"
	"log"
)

func init() {
	log.SetPrefix("blockchain: ")
}

func main() {
	port := flag.Uint("port", 5001, "port number for blockchain server")
	flag.Parse()
	app := NewBlockchainServer(uint16(*port))
	app.Run()
}
