package gol

import (
	"fmt"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

/*
Distributed part (2)
	Note that the shared memory solution for Median Filter should be used
*/

// //constants
const aliveCellsPollDelay = 2 * time.Second

func sendWriteCommand(p Params, c distributorChannels, currentTurn int, currentWorld [][]byte) {

	filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, currentTurn)
	c.ioCommand <- ioOutput
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- currentWorld[y][x]
		}
	}

	c.events <- ImageOutputComplete{CompletedTurns: currentTurn, Filename: filename}
}

func finishServer(client *rpc.Client, c distributorChannels){
	res := new(stubs.QuitWorldResponse)
	err := client.Call(stubs.BrokerFinishHander, stubs.EmptyRequest{}, res)
	if err != nil {
		fmt.Printf("Error client couldn't Finish server %s\n", err)
	}

	c.events <- FinalTurnComplete{CompletedTurns: res.OnTurn, Alive: res.Alive}
}

func kill(client *rpc.Client, c distributorChannels) {
	res := new(stubs.KillBrokerResponse)

	client.Call(stubs.KillBroker, stubs.EmptyRequest{}, res)

	c.events <- FinalTurnComplete{CompletedTurns: res.OnTurn, Alive: res.Alive}
}

var paused sync.Mutex

//we only ever need write to events, and read from turns
func ticks(c distributorChannels, broker *rpc.Client, done <-chan bool) {
	//newRound :=
	ticker := time.NewTicker(aliveCellsPollDelay)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			req := stubs.EmptyRequest{}

			res := new(stubs.AliveResponse)

			broker.Call(stubs.BrokerAliveHandler, req, res)
			c.events <- AliveCellsCount{CompletedTurns: res.OnTurn, CellsCount: len(res.Alive)}
		}
	}
}

func handleKeyPresses(p Params, c distributorChannels, client *rpc.Client, keyPresses <-chan rune, killServer chan<- bool) {
	isPaused := false
	for {
		k := <-keyPresses
		switch k {
		case 's':
			//request current state through stubs package
			//write the pgm out
			req := stubs.EmptyRequest{}
			res := new(stubs.WorldResponse)

			remoteDone := make(chan *rpc.Call, 1)
			call := client.Go(stubs.SaveWorldHandler, req, res, remoteDone)
			<-call.Done
			fmt.Println("Generating PGM")
			sendWriteCommand(p, c, res.OnTurn, res.World)
			fmt.Println("Generated PGM")
		case 'q':
			fmt.Println("Closing the controller client program")
			//leave the server running
			finishServer(client, c)
			return
		case 'k':
			//request closure of server through stubs package
			fmt.Println("Closing all components of the distributed system")
			kill(client, c)
			killServer <- true
			return
		case 'p':
			//request pausing of aws node through stubs package
			//then print the current turn
			//once p is pressed again resume processing through requesting from stubs
			if(!isPaused){
				paused.Lock()
				donePause := make(chan *rpc.Call, 1)
				pauseRes := new(stubs.PauseResponse)
				doPause := client.Go(stubs.BrokerPauseHandler, stubs.PauseRequest{Pause: true}, pauseRes, donePause)
				<-doPause.Done
				isPaused = true
				c.events <-StateChange{CompletedTurns: pauseRes.Turns, NewState: Paused}
			}else{
				donePause := make(chan *rpc.Call, 1)
				pauseRes := new(stubs.PauseResponse)
				doPause := client.Go(stubs.BrokerPauseHandler, stubs.PauseRequest{Pause: false}, pauseRes, donePause)
				<-doPause.Done
				isPaused = false
				c.events <-StateChange{CompletedTurns: pauseRes.Turns, NewState: Executing}
				paused.Unlock()
			}

		default:

		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune, client *rpc.Client, cont bool) {
	// TODO: Create a 2D slice to store the world.

	c.ioCommand <- ioInput //send the appropriate command... (jump ln155)
	filename := fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)

	c.ioFilename <- filename //...then send to distributor channel

	world := make([][]byte, p.ImageHeight)


	for y := 0; y < p.ImageHeight; y++ {
		world[y] = make([]byte, p.ImageWidth)
		for x := 0; x < p.ImageWidth; x++ {
			pixel := <-c.ioInput //gets image in with the io.goroutine
			world[y][x] = pixel
		}
	}

	killServer := make(chan bool, 1)
	go handleKeyPresses(p, c, client, keyPresses, killServer)

	done := make(chan bool)
	go ticks(c, client, done)


	params := stubs.Params{Turns: p.Turns, Threads: p.Threads, ImageWidth: p.ImageWidth, ImageHeight: p.ImageHeight}

	brokerReq := stubs.NewClientRequest{World: world, Params: params, Continue: cont}
	brokerRes := new(stubs.NewClientResponse)

	client.Call(stubs.ClientHandler, brokerReq, brokerRes)
	// TODO: Report the final state using FinalTurnCompleteEvent.

	final := FinalTurnComplete{CompletedTurns: brokerRes.Turns, Alive: brokerRes.Alive}
	
	c.events <- final //sending event down events channel
	//
	sendWriteCommand(p, c, brokerRes.Turns, brokerRes.World)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	//c.events <- StateChange{turns, Quitting} //passed in the total turns complete as being that which we set out to complete, as otherwise we would have errored
	//
	done <- true
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
