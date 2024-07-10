package main

import (
	"flag"
	"fmt"
	"log"
)

func init() {
	log.SetPrefix("blockchain: ")
}

func main() {
	port := flag.Uint("port", 5001, "port number for blockchain server")
	flag.Parse()
	fmt.Printf("portは%dです\n", *port)
	app := NewBlockchainServer(uint16(*port))
	app.Run()
}
