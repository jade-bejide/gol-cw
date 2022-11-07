package gol

import (
	"strconv"
	"sync"
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
//counts the number of alive neighbours of a given cell
func countLiveNeighbours(p Params, x int, y int, world [][]byte) int {
	liveNeighbours := 0

	w := p.ImageWidth - 1
	h := p.ImageHeight - 1

	l := x - 1
	r := x + 1
	u := y + 1
	d := y - 1

	if l < 0 {
		l = w
	}
	if r > w {
		r = 0
	}
	if u > h {
		u = 0
	}
	if d < 0 {
		d = h
	}

	if world[u][x] == 255 {
		liveNeighbours += 1
	}
	if world[d][x] == 255 {
		liveNeighbours += 1
	}
	if world[u][l] == 255 {
		liveNeighbours += 1
	}
	if world[u][r] == 255 {
		liveNeighbours += 1
	}
	if world[d][l] == 255 {
		liveNeighbours += 1
	}
	if world[d][r] == 255 {
		liveNeighbours += 1
	}
	if world[y][l] == 255 {
		liveNeighbours += 1
	}
	if world[y][r] == 255 {
		liveNeighbours += 1
	}

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

//completes one turn of gol
func calculateNextState(p Params, world [][]byte, nextWorld [][]byte, y1 int, y2 int) {
	x := 0

	height := y2 - y1

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

			j += 1
		}
		x += 1
	}
}

func spreadWorkload(h int, threads int) []int {
	splits := make([]int, threads+1)

	splitSize := h / threads
	extraRows := h % threads

	index := 0
	for i := 0; i < h; i += splitSize {
		splits[index] = i

		//if a worker needs to take on extra rows (this will be at most one row by modulo law)
		//add 1 to shuffle along accordingly
		if extraRows > 0 && i > 0 {
			splits[index]++
			extraRows--
			i++
		}
	}

	return splits
}

func worker(p Params, y1, y2 int, lastWorld, nextWorld [][]uint8, wg *sync.WaitGroup) {
	//do the thgings
	calculateNextState(p, lastWorld, nextWorld, y1, y2)
	wg.Done()
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

	var wg sync.WaitGroup

	nextWorld := saveWorld(world)
	splits := spreadWorkload(len(world), p.Threads)

	turn := 0
	// TODO: Execute all turns of the Game of Life.

	for turn = 0; turn < p.Turns; turn++ {
		for i := 0; i < p.Threads; i++ {
			wg.Add(1)
			go worker(p, splits[i], splits[i+1], world, nextWorld, &wg)
		}
		wg.Wait() //waits for all threads to be done
		world = nextWorld
	}
	// TODO: Report the final state using FinalTurnCompleteEvent.

	aliveCells := calculateAliveCells(p, world)
	final := FinalTurnComplete{CompletedTurns: p.Turns, Alive: aliveCells}

	c.events <- final //sending event down events channel

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
