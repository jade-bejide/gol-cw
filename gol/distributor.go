package gol

import (
	"fmt"
	"strconv"
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

//constants
const aliveCellsPollDelay = 2 * time.Second

type Turns struct { //pass-by-ref integer
	T   int
	mut sync.Mutex
}

type SharedWorld struct { //pass-by-ref world
	W   [][]uint8
	mut sync.Mutex
}

//returns a closure of a 2d array of uint8s
func makeImmutableMatrix(m [][]uint8) func(x, y int) uint8 {
	return func(x, y int) uint8 {
		return m[y][x]
	}
}

var turns Turns
var currentWorld SharedWorld

type Turns struct {
	T int
	mut sync.Mutex
}

type SharedWorld struct {
	W [][]uint8
	mut sync.Mutex
}

//counts the number of alive neighbours of a given cell
func countLiveNeighbours(p Params, x int, y int, world [][]byte) int {
	liveNeighbours := 0

	w := p.ImageWidth - 1
	h := p.ImageHeight - 1

	l := x - 1
	r := x + 1
	u := y + 1
	d := y - 1

	if l < 0 { l = w }
	if r > w { r = 0 }
	if u > h { u = 0 }
	if d < 0 { d = h }

	if world[u][x] == 255 { liveNeighbours += 1 }
	if world[d][x] == 255 { liveNeighbours += 1 }
	if world[u][l] == 255 { liveNeighbours += 1 }
	if world[u][r] == 255 { liveNeighbours += 1 }
	if world[d][l] == 255 { liveNeighbours += 1 }
	if world[d][r] == 255 { liveNeighbours += 1 }
	if world[y][l] == 255 { liveNeighbours += 1 }
	if world[y][r] == 255 { liveNeighbours += 1 }

	return liveNeighbours
}

//updates the state of a world
func updateState(isAlive bool, neighbours int) bool {
	return isAlive && neighbours > 1 && neighbours < 4 || !isAlive && neighbours == 3
}

//checks if a cell is alive
func isAlive(x int, y int, world [][]byte) bool {
	return world[y][x] != 0
}

//makes a deep copy of a previous world state
func saveWorld(world [][]byte) [][]byte {
	cp := make([][]byte, len(world))

	for i := range world {
		cp[i] = make([]byte, len(world[i]))
		copy(cp[i], world[i])
	}

	return cp
}

//creates a 2D slice of a world of size height x width
func genWorldBlock(height int, width int) [][]byte {
	worldBlock := make([][]byte, height)

	for i := range worldBlock {
		worldBlock[i] = make([]byte, width)
	}

	return worldBlock
}

type WorldBlock struct {
	Data  [][]byte
	Index int
}

//completes one turn of gol
func calculateNextState(p Params, c distributorChannels,  world[][]byte, y1 int, y2 int, turn int) [][]byte {
	x := 0

	height := y2 - y1

	nextWorld := genWorldBlock(height, p.ImageWidth)

	for x < p.ImageWidth {
		j := y1
		for y := 0; y < height; y++ {
			neighbours := countLiveNeighbours(p, x, j, world)
			alive := isAlive(x, j, world)

			alive = updateState(alive, neighbours)

			if alive {
				nextWorld[y][x] = 255
			} else {
				nextWorld[y][x] = 0
			}
			if world[j][x] != nextWorld[y][x] {
				cell := Cell{X: x, Y: j}
				c.events <- CellFlipped{CompletedTurns: turn, Cell: cell}
			}

			j += 1
		}
		x += 1
	}

	return nextWorld
}

func spreadWorkload(h int, threads int) []int {
    splits := make([]int, threads +1)

	splitSize := h / threads
	extraRows := h % threads

	index := 0
	for i := 0; i <= h; i += splitSize {
		splits[index] = i

		//if a worker needs to take on extra rows (this will be at most one row by modulo law)
		//add 1 to shuffle along accordingly
		if extraRows > 0 && i > 0 {
			splits[index]++
			extraRows--
			i++
		}
		index++
	}
	return splits
}

func worker(p Params, y1, y2 int, lastWorld [][]uint8, workerId int, outCh chan<- WorldBlock) {
	//do the things
	nextWorld := calculateNextState(p, lastWorld, y1, y2)
	outCh <- WorldBlock{Index: workerId, Data: nextWorld}
}

//traverses the world and takes the coordinates of any alive cells
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	x := 0
	y := 0

	var cells []util.Cell

	for x < p.ImageWidth {
		y = 0
		for y < p.ImageHeight {
			if isAlive(x, y, world) {
				c := util.Cell{x, y}
				cells = append(cells, c)
			}
			y += 1
		}
		x += 1
	}

	return cells
}


//we only ever need write to events, and read from turns
func ticks(p Params, events chan<- Event, turns *Turns, world *SharedWorld, pollRate time.Duration) {
	ticker := time.NewTicker(pollRate)
	for {
		<-ticker.C
		//turns.mut.Lock()
		//critical section, we want to report while calculation is paused
		world.mut.Lock()
		events <- AliveCellsCount{turns.T, len(calculateAliveCells(p, world.W))}
		world.mut.Unlock()
		//turns.mut.Unlock()
    }
}


func sendWriteCommand(p Params, c distributorChannels, currentTurn int, currentWorld [][]byte) {
	filename := strconv.Itoa(p.ImageWidth) + "x"  + strconv.Itoa(p.ImageHeight)
	c.ioFilename <- filename
	c.ioCommand <- ioOutput

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- currentWorld[y][x]
		}
	}
}

func handleSDL(p Params, c distributorChannels, keyPresses <-chan rune) {
	var paused bool
	paused = false
	for {
		keyPress := <-keyPresses
		switch keyPress {
		case 'p':
			if !paused {
				turn.mut.Lock()
					c.events <- StateChange{CompletedTurns: turn.T, NewState: Paused}
					currentWorld.mut.Lock()
						sendWriteCommand(p, c, turn.T, currentWorld.W)
					currentWorld.mut.Unlock()
				turn.mut.Unlock()
				paused = true
			} else {
				turn.mut.Lock()
					c.events <- StateChange{CompletedTurns: turn.T, NewState: Executing}
				turn.mut.Unlock()
				fmt.Println("Continuing")
				paused = false
			}

		case 's':
			sendWriteCommand(p, c, turn.T, world)
		case 'q':
			turn.mut.Lock()
				c.events <- StateChange{CompletedTurns: turn.T, NewState: Quitting}
				currentWorld.mut.Lock()
					sendWriteCommand(p, c, turn.T, currentWorld.W)
				currentWorld.mut.Unlock()
			turn.mut.Unlock()

		}
  }
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.

	c.ioCommand <- ioInput //send the appropriate command... (jump ln155)
	filename := ""
	wStr := strconv.Itoa(p.ImageWidth)
	hStr := strconv.Itoa(p.ImageHeight)
	filename += wStr
	filename += "x"
	filename += hStr

	c.ioFilename <- filename //...then send to distributor channel

	world := make([][]byte, p.ImageHeight)

	for y := 0; y < p.ImageHeight; y++ {
		world[y] = make([]byte, p.ImageWidth)
		for x := 0; x < p.ImageWidth; x++ {
			pixel := <-c.ioInput //gets image in with the io.goroutine
			world[y][x] = pixel
		}
	}

	splits := spreadWorkload(len(world), p.Threads)
	turn := 0
	outCh := make(chan WorldBlock)
	// TODO: Execute all turns of the Game of Life.

	//ticker tools
	sharedTurns := Turns{0, sync.Mutex{}}
	sharedWorld := SharedWorld{world, sync.Mutex{}}
	go ticks(p, c.events, &sharedTurns, &sharedWorld, aliveCellsPollDelay)

	//sharedTurns.mut.Lock()

	for turn = 0; turn < p.Turns; turn++ {

		for i := 0; i < p.Threads; i++ {
			go worker(p, splits[i], splits[i+1], world, i, outCh)
		}

		nextWorld := make([][][]byte, p.Threads)

		for i := 0; i < p.Threads; i++ {
			section := <-outCh
			nextWorld[section.Index] = section.Data
		}

		sharedWorld.mut.Lock()
		world = make([][]byte, 0)
		for _, section := range nextWorld {
			for _, row := range section {
				world = append(world, row)
			}
		}
		sharedWorld.mut.Unlock()

		c.events <- TurnComplete{turn}
		sharedTurns.T++
		sharedWorld.W = world

		c.events <- TurnComplete{CompletedTurns: turn}
	}
	//sharedTurns.mut.Unlock()
	// TODO: Report the final state using FinalTurnCompleteEvent.

	aliveCells := calculateAliveCells(p, world)
	final := FinalTurnComplete{CompletedTurns: p.Turns, Alive: aliveCells}

	c.events <- final //sending event down events channel

   	sendWriteCommand(p, c, p.Turns, world)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
