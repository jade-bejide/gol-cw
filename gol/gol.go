package gol

import (
	"net/rpc"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune, cont bool) {
	/*
		inputs
			p -> CL arguments
			events ->
	*/

	//	TODO: Put the missing channels in here.

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string)
	ioOutput := make(chan uint8)
	ioInput := make(chan uint8)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}

	//entrypoint of the io.go goroutine
	go startIo(p, ioChannels) //where the io goroutine is started

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   ioOutput,
		ioInput:    ioInput,
	}
	server := "localhost:8031" //to dial the broker
	//adding rpc "server" to make call for work to ()
	client, err := rpc.Dial("tcp", server)
	if(err != nil) { panic(err) } //rudimentary error handling
	defer client.Close()

	distributor(p, distributorChannels, keyPresses, client, cont)
}
