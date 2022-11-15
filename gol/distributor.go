package gol

import (
	"fmt"
	"net/rpc"
	_ "sync"
	"time"
	_ "time"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
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

// func spreadWorkload(h int, threads int) []int {
// 	splits := make([]int, threads+1)

// 	splitSize := h / threads
// 	extraRows := h % threads

// 	index := 0
// 	for i := 0; i <= h; i += splitSize {
// 		splits[index] = i

// 		//if a worker needs to take on extra rows (this will be at most one row by modulo law)
// 		//add 1 to shuffle along accordingly
// 		if extraRows > 0 && i > 0 {
// 			splits[index]++
// 			extraRows--
// 			i++
// 		}
// 		index++
// 	}
// 	return splits
// }




//we only ever need write to events, and read from turns
func ticks(c distributorChannels, client *rpc.Client, done <-chan bool) {
	//newRound :=
	ticker := time.NewTicker(aliveCellsPollDelay)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			req := stubs.EmptyRequest{}

			res := new(stubs.AliveResponse)
			//func (client *Client) Go(serviceMethod string, args any, reply any, done chan *Call) *Call

			done := make(chan *rpc.Call, 1)
			callRes := client.Go(stubs.AliveHandler, req, res, done)
			<-callRes.Done
			c.events <- AliveCellsCount{CompletedTurns: res.OnTurn, CellsCount: res.Alive}
		}
	}
}

func sendWriteCommand(p Params, c distributorChannels, currentTurn int, currentWorld [][]byte) {
	//fmt.Printf("final %v; called on %v\n", p.Turns, currentTurn)

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

func handleKeyPresses(p Params, c distributorChannels, client *rpc.Client, keyPresses <-chan rune){
	isPaused := false
	for {
		k := <-keyPresses
		switch k {
		case 's':
			//request current state through stubs package
			//write the pgm out
			req := stubs.EmptyRequest{}
			res := new(stubs.Response)

			remoteDone := make(chan *rpc.Call, 1)
			call := client.Go(stubs.PollWorldHandler, req, res, remoteDone)
			<-call.Done
			//fmt.Println("CALL FINSIHED FROM KEYPRESSER ", call.ServiceMethod)
			//fmt.Println("RESPONSE TURNS", res, "REPLY TURNS", call.Reply)

			fmt.Println("Generating PGM")
			sendWriteCommand(p, c, res.Turn, res.World)
			fmt.Println("Generated PGM")
		case 'q':
			fmt.Println("Shutting down local component")
			err := client.Call(stubs.ResetHandler, stubs.EmptyRequest{}, new(stubs.EmptyResponse))
			if err != nil {
				panic(err)
			}
			return
		case 'k':
			//request closure of server through stubs package
			fmt.Println("Shutting down remote and local components")

		case 'p':
			//request pausing of aws node through stubs package
			//then print the current turn
			//once p is pressed again resume processing through requesting from stubs
			if(!isPaused){
				fmt.Println("Paused")
				isPaused = true
			}else{
				fmt.Println("Continuing")
				isPaused = false
			}

		default:

		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune, client *rpc.Client) {
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

	//doneTakeTurns  := make(chan bool)
	go handleKeyPresses(p, c, client, keyPresses)

    req := stubs.Request{World: world, Params: stubs.Params(p)}
    res := new(stubs.Response)

	done := make(chan bool)
	go ticks(c, client, done)

	remoteDone := make(chan *rpc.Call, 1)
    call := client.Go(stubs.TurnsHandler, req, res, remoteDone)

	var alive []util.Cell
	var turns int
	//var emptyReq stubs.EmptyRequest
	//var worldRes *stubs.Response

	<-call.Done
	world = res.World
	alive = res.Alive
	turns = res.Turn

	// TODO: Report the final state using FinalTurnCompleteEvent.

	final := FinalTurnComplete{CompletedTurns: turns, Alive: alive}

	c.events <- final //sending event down events channel

	sendWriteCommand(p, c, p.Turns, world)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
  
	c.events <- StateChange{p.Turns, Quitting} //passed in the total turns complete as being that which we set out to complete, as otherwise we would have errored

	done <- true
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
