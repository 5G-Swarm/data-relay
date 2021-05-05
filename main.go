package main

import (
	"data_relay/server"
)

// Block the main function
var done = make(chan struct{})

func main() {
	server.Run()
	<-done
}
