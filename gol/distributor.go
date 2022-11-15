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

// i believe the following commented merge had all been deleted in jade's branch, which is fine, but just incase i was wrong its still here, just commented

// <<<<<<< feature-server //my incoming branch 
// //constants
const aliveCellsPollDelay = 2 * time.Second

// //type Boolean struct {
// //	B bool
// //
// //}

// type Turns struct { //pass-by-ref integer
// 	T   int
// 	mut sync.Mutex
// }

// type SharedWorld struct { //pass-by-ref world
// 	W   [][]uint8
// 	mut sync.Mutex
// }

// //returns a closure of a 2d array of uint8s
// func makeImmutableMatrix(m [][]uint8) func(x, y int) uint8 {
// 	return func(x, y int) uint8 {
// 		return m[y][x]
// 	}
// }

// //counts the number of alive neighbours of a given cell
// func countLiveNeighbours(p Params, x int, y int, world [][]byte) int {
// 	liveNeighbours := 0

// 	w := p.ImageWidth - 1
// 	h := p.ImageHeight - 1

// 	l := x - 1
// 	r := x + 1
// 	u := y + 1
// 	d := y - 1

// 	if l < 0 {
// 		l = w
// 	}
// 	if r > w {
// 		r = 0
// 	}
// 	if u > h {
// 		u = 0
// 	}
// 	if d < 0 {
// 		d = h
// 	}

// 	if world[u][x] == 255 {
// 		liveNeighbours += 1
// 	}
// 	if world[d][x] == 255 {
// 		liveNeighbours += 1
// 	}
// 	if world[u][l] == 255 {
// 		liveNeighbours += 1
// 	}
// 	if world[u][r] == 255 {
// 		liveNeighbours += 1
// 	}
// 	if world[d][l] == 255 {
// 		liveNeighbours += 1
// 	}
// 	if world[d][r] == 255 {
// 		liveNeighbours += 1
// 	}
// 	if world[y][l] == 255 {
// 		liveNeighbours += 1
// 	}
// 	if world[y][r] == 255 { liveNeighbours += 1 }

// 	return liveNeighbours
// }

// //updates the state of a world
// func updateState(isAlive bool, neighbours int) bool {
// 	return isAlive && neighbours > 1 && neighbours < 4 || !isAlive && neighbours == 3
// }

// //checks if a cell is alive
// func isAlive(x int, y int, world [][]byte) bool {
// 	return world[y][x] != 0
// }

// //makes a deep copy of a previous world state
// func saveWorld(world [][]byte) [][]byte {
// 	cp := make([][]byte, len(world))

// 	for i := range world {
// 		cp[i] = make([]byte, len(world[i]))
// 		copy(cp[i], world[i])
// 	}

// 	return cp
// }

// //creates a 2D slice of a world of size height x width
// func genWorldBlock(height int, width int) [][]byte {
// 	worldBlock := make([][]byte, height)

// 	for i := range worldBlock {
// 		worldBlock[i] = make([]byte, width)
// 	}

// 	return worldBlock
// }

// type WorldBlock struct {
// 	Data  [][]byte
// 	Index int
// }

// ////completes one turn of gol
// //func calculateNextState(p Params, c distributorChannels, world [][]byte, y1 int, y2 int, turn int) [][]byte {
// //	x := 0
// //
// //	height := y2 - y1
// //
// //	nextWorld := genWorldBlock(height, p.ImageWidth)
// //
// //	for x < p.ImageWidth {
// //		j := y1
// //		for y := 0; y < height; y++ {
// //			neighbours := countLiveNeighbours(p, x, j, world)
// //			alive := isAlive(x, j, world)
// //
// //			alive = updateState(alive, neighbours)
// //
// //			if alive {
// //				nextWorld[y][x] = 255
// //			} else {
// //				nextWorld[y][x] = 0
// //			}
// //			if world[j][x] != nextWorld[y][x] {
// //				cell := util.Cell{X: x, Y: j}
// //				c.events <- CellFlipped{CompletedTurns: turn, Cell: cell}
// //			}
// //
// //			j += 1
// //		}
// //		x += 1
// //	}
// //
// //	return nextWorld
// //}

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

// func worker(p Params, c distributorChannels, turn int, y1 int, y2 int, lastWorld [][]uint8, workerId int, outCh chan<- WorldBlock) {
// 	//do the things
// 	nextWorld := calculateNextState(p, c, lastWorld, y1, y2, turn)
// 	outCh <- WorldBlock{Index: workerId, Data: nextWorld}
// }

// //traverses the world and takes the coordinates of any alive cells
// //func calculateAliveCells(p Params, world [][]byte) []util.Cell {
// //	x := 0
// //	y := 0
// //
// //	var cells []util.Cell
// //
// //	for x < p.ImageWidth {
// //		y = 0
// //		for y < p.ImageHeight {
// //			if isAlive(x, y, world) {
// //				c := util.Cell{x, y}
// //				cells = append(cells, c)
// //			}
// //			y += 1
// //		}
// //		x += 1
// //	}
// //
// //	return cells
// //}

// =======



//we only ever need write to events, and read from turns
func ticks(c distributorChannels, client *rpc.Client, done <-chan bool) {
	//newRound :=
	ticker := time.NewTicker(aliveCellsPollDelay)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			req := stubs.AliveRequest{}

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

func handleKeyPresses(p Params, c distributorChannels, client *rpc.Client, keyPresses <-chan rune, done <-chan bool, doneTurns chan<- bool){
	isPaused := false
	for {
		select{
			case <-done:
				return

			case k := <-keyPresses:
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

					break
				case 'q':
					//close controller
					doneTurns <- true
					return
				case 'k':
					//request closure of server through stubs package
					fmt.Println("Shutting down remote and local components")
					break
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
					break
				default:
					break
				}
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

	doneKeyPresses := make(chan bool)
	doneTakeTurns  := make(chan bool)
	go handleKeyPresses(p, c, client, keyPresses, doneKeyPresses, doneTakeTurns)

    req := stubs.Request{World: world, Params: stubs.Params(p)}
    res := new(stubs.Response)

	done := make(chan bool)
	go ticks(c, client, done)

	remoteDone := make(chan *rpc.Call, 1)
    call := client.Go(stubs.TurnsHandler, req, res, remoteDone)

	var alive []util.Cell
	var turns int
	var emptyReq stubs.EmptyRequest
	var worldRes *stubs.Response

	select {
		case <-call.Done: //we terminate on finishing the number of turns
			doneKeyPresses <- true //tell keypresses to stop
			world = res.World
			alive = res.Alive
			turns = p.Turns
		case <-doneTakeTurns: //we quit early, and poll the world once more to exit correctly
			//poll
			emptyReq = stubs.EmptyRequest{}
			worldRes = new(stubs.Response)
			client.Call(stubs.PollWorldHandler, emptyReq, worldRes)
			//...then shutdown
			fmt.Println("Shutting down local component")
			err := client.Call(stubs.ResetHandler, stubs.EmptyRequest{}, new(stubs.EmptyResponse))
			if err != nil {
				panic(err)
			}

			world = worldRes.World
			alive = worldRes.Alive
			turns = worldRes.Turn
	}

	//fmt.Println("CALL FINISHED ", call.ServiceMethod)

    // finalTurns := res.Turns       this property was unused, just need to avoid errors we shall add it back later

//     assert p.Turns == finalTurns

// 	go handleSDL(p, c, keyPresses, &sharedTurns, &sharedWorld, &pauseLock)

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
