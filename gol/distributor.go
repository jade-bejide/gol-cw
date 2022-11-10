package gol

import (
	"fmt"
	"sync"
	"time"
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

//we only ever need write to events, and read from turns
// func ticks(p Params, events chan<- Event, turns *Turns, world *SharedWorld, pollRate time.Duration) {
// 	ticker := time.NewTicker(pollRate)
// 	for {
// 		select {
// 		case <-done:
// 			return
// 		case <-ticker.C:
// 			//critical section, we want to report while calculation is paused
// 			world.mut.Lock()
// 			events <- AliveCellsCount{turns.T, len(calculateAliveCells(p, world.W))}
// 			world.mut.Unlock()
// 		}
// 	}
// }

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

// func handleSDL(p Params, c distributorChannels, keyPresses <-chan rune, turns *Turns, world *SharedWorld, pauseLock *sync.Mutex) {
// 	paused := false
// 	for {
// 		keyPress := <-keyPresses
// 		switch keyPress {
// 		case 'p':
// 			fmt.Println("P")
// 			if !paused {
// 				turns.mut.Lock()
// 				c.events <- StateChange{CompletedTurns: turns.T, NewState: Paused}
// 				turns.mut.Unlock()
// 				world.mut.Lock()
//
// 				sendWriteCommand(p, c, turns.T, world.W)
//
// 				paused = true
// 			} else {
// 				turns.mut.Lock()
// 				c.events <- StateChange{CompletedTurns: turns.T, NewState: Executing}
// 				turns.mut.Unlock()
//
// 				world.mut.Unlock()
//
// 				fmt.Println("Continuing")
// 				paused = false
// 			}
// 		case 's':
// 			sendWriteCommand(p, c, turns.T, world.W)
// 		case 'q':
// 			turns.mut.Lock()
// 			c.events <- StateChange{CompletedTurns: turns.T, NewState: Quitting}
// 			world.mut.Lock()
// 			sendWriteCommand(p, c, turns.T, world.W)
// 			c.events <- FinalTurnComplete{CompletedTurns: turns.T, Alive: calculateAliveCells(p, world.W)}
// 			world.mut.Unlock()
// 			turns.mut.Unlock()
// 		default:
//
// 		}
// 	}
// }


// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
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


    req := stubs.Request{World: world, Turns: turns}
    res := new(stubs.Response)
    client.Call(stubs.Method, req, res)

    world = res.World
    alive := res.Alive
    finalTurns := res.Turns

//     assert p.Turns == finalTurns
// 	go ticks(p, c.events, &sharedTurns, &sharedWorld, aliveCellsPollDelay)
// 	go handleSDL(p, c, keyPresses, &sharedTurns, &sharedWorld, &pauseLock)

	// TODO: Report the final state using FinalTurnCompleteEvent.

	final := FinalTurnComplete{CompletedTurns: p.Turns, Alive: alive}

	c.events <- final //sending event down events channel
	sendWriteCommand(p, c, p.Turns, world)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	done <- true
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
