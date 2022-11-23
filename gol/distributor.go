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

type Gol struct {
	Turns int
	TurnsMut sync.Mutex
	WorldA [][]uint8 //two alternating worlds to avoid continued memory allocations
	WorldB [][]uint8
	World *[][]uint8
	Next *[][]uint8
	WorldMut sync.Mutex
	WorkGroup sync.WaitGroup
}

type Worker struct {
	ID int
	TopY int
	EndY int
	WorldStrip [][]uint8 //a strip that belongs to a worker, it will write its output here to avoid races
}

//swap works
func (g *Gol) swapWorld(p Params) {
	g.WorldMut.Lock()
	if g.World == &g.WorldA {
		g.World = &g.WorldB
		g.Next = &g.WorldA
	}else if g.World == &g.WorldB{
		g.World = &g.WorldA
		g.Next = &g.WorldB
	}
	g.WorldMut.Unlock()
}

func newGol(width, height, threads int, splits []int, initialWorld [][]uint8) *Gol{

	g := Gol{
		Turns: 0,
		TurnsMut: sync.Mutex{},
		WorldA: initialWorld,
		WorldB: genWorldBlock(width, height),
		World: nil,
		Next: nil,
		WorldMut: sync.Mutex{},
	};

	g.World = &g.WorldA //these swap each turn
	g.Next  = &g.WorldB

	return &g
}

func newWorkers(p Params, splits []int) []Worker {
	var workers []Worker

	for i := 0; i < p.Threads; i++ {
		strip := genWorldBlock(splits[i+1] - splits[i], p.ImageWidth)
		workers = append(workers, Worker{
			ID: i,
			TopY: splits[i],
			EndY: splits[i+1],
			WorldStrip: strip, //a strip for each worker (to write in)
		})
	}

	return workers
}

//returns a closure of a 2d array of uint8s
func makeImmutableMatrix(m [][]uint8) func(x, y int) uint8 {
	return func(x, y int) uint8 {
		return m[y][x]
	}
}

//counts the number of alive neighbours of a given cell
func countLiveNeighbours(p Params, x int, y int, world func(x, y int) uint8) int {
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

	if world(x, u) == 255 {
		liveNeighbours += 1
	}
	if world(x, d) == 255 {
		liveNeighbours += 1
	}
	if world(l, u) == 255 {
		liveNeighbours += 1
	}
	if world(r, u) == 255 {
		liveNeighbours += 1
	}
	if world(l, d) == 255 {
		liveNeighbours += 1
	}
	if world(r, d) == 255 {
		liveNeighbours += 1
	}
	if world(l, y)  == 255 {
		liveNeighbours += 1
	}
	if world(r, y) == 255 {
		liveNeighbours += 1
	}

	return liveNeighbours
}

//updates the state of a world
func updateState(isAlive bool, neighbours int) bool {
	return isAlive && neighbours > 1 && neighbours < 4 || !isAlive && neighbours == 3
}

//checks if a cell is alive
func isAlive(x int, y int, world func(x, y int) uint8) bool {
	return world(x, y) != 0
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

func (w *Worker) run(p Params, c distributorChannels, world func(x, y int) uint8, wg *sync.WaitGroup, turn int) {
	w.calculateNextState(p, c, world, w.WorldStrip, turn)
	defer wg.Done()
}

//completes one turn of gol
// 'world' argument is the world we read from, strip is where we put our new data
func (w *Worker) calculateNextState(p Params, c distributorChannels, world func(x, y int) uint8, strip [][]uint8, turn int) {
	x := 0
	height := w.EndY - w.TopY

	for x < p.ImageWidth {
		j := w.TopY
		for y := 0; y < height; y++ {
			neighbours := countLiveNeighbours(p, x, j, world) //reading an immutable world needs no mutex
			alive := isAlive(x, j, world)

			alive = updateState(alive, neighbours)

			if alive {
				strip[y][x] = 255 //writing needs no mutex
			} else {
				strip[y][x] = 0
			}
			if world(x, j) != strip[y][x] {
				cell := util.Cell{X: x, Y: j}
				c.events <- CellFlipped{CompletedTurns: turn, Cell: cell}
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

//traverses the world and takes the coordinates of any alive cells
func calculateAliveCells(p Params, world func(x, y int) uint8) []util.Cell {
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
func (g *Gol) ticks(p Params, events chan<- Event, pollRate time.Duration) {
	ticker := time.NewTicker(pollRate)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			//critical section, we want to report while calculation is paused
			g.WorldMut.Lock()
			g.TurnsMut.Lock()
			events <- AliveCellsCount{g.Turns, len(calculateAliveCells(p, makeImmutableMatrix(*g.World)))}
			g.TurnsMut.Unlock()
			g.WorldMut.Unlock()
		}
	}
}

func (g *Gol) handleSDL(p Params, c distributorChannels, keyPresses <-chan rune, pauseLock *sync.Mutex) {
	paused := false
	for {
		keyPress := <-keyPresses
		switch keyPress {
		case 'p':
			//fmt.Println("P")
			if !paused {
				g.TurnsMut.Lock()
				c.events <- StateChange{CompletedTurns: g.Turns, NewState: Paused}
				g.TurnsMut.Unlock()
				g.WorldMut.Lock()

				sendWriteCommand(p, c, g.Turns, *g.World)

				paused = true
			} else {
				g.TurnsMut.Lock()
				c.events <- StateChange{CompletedTurns: g.Turns, NewState: Executing}
				g.TurnsMut.Unlock()

				g.WorldMut.Unlock()

				fmt.Println("Continuing")
				paused = false
			}
		case 's':
			g.TurnsMut.Lock()
			g.WorldMut.Lock()
			sendWriteCommand(p, c, g.Turns, *g.World)
			g.WorldMut.Unlock()
			g.TurnsMut.Unlock()
		case 'q':
			g.TurnsMut.Lock()
			c.events <- StateChange{CompletedTurns: g.Turns, NewState: Quitting}
			g.WorldMut.Lock()
			sendWriteCommand(p, c, g.Turns, *g.World)
			c.events <- FinalTurnComplete{CompletedTurns: g.Turns, Alive: calculateAliveCells(p, makeImmutableMatrix(*g.World))}
			g.WorldMut.Unlock()
			g.TurnsMut.Unlock()
		default:

		}
	}
}

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

var done chan bool

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
			if pixel == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{x, y}}
			}
		}
	}

	splits := spreadWorkload(len(world), p.Threads)

	// TODO: Execute all turns of the Game of Life.

	g := newGol(p.ImageWidth, p.ImageHeight, p.Threads, splits, world)
	workers := newWorkers(p, splits)


	pauseLock := sync.Mutex{}
	done = make(chan bool)
	go g.ticks(p, c.events, aliveCellsPollDelay)
	go g.handleSDL(p, c, keyPresses, &pauseLock)

	g.TurnsMut.Lock()
	for g.Turns = 0; g.Turns < p.Turns; g.Turns++ {
		g.TurnsMut.Unlock()
		g.WorkGroup.Add(p.Threads)
		for i := 0; i < p.Threads; i++ {
			go workers[i].run(p, c, makeImmutableMatrix(*g.World), &g.WorkGroup, i)
		}
		g.WorkGroup.Wait()

		g.WorldMut.Lock()
		for i := 0; i < len(workers); i++{
			//for each worker we want to put its slice back into the world, we can set the world row by row
			offset := workers[i].TopY
			for y := 0; y < len(workers[i].WorldStrip); y++ { //we must set pixel by pixel, copying through any other means is the same number of operations
				for x := 0; x < p.ImageWidth; x++ {
					(*g.Next)[offset + y][x] = workers[i].WorldStrip[y][x]
				}

			}
		}
		g.WorldMut.Unlock()

		g.TurnsMut.Lock()
		c.events <- TurnComplete{g.Turns}
		g.swapWorld(p)
	}
	g.TurnsMut.Unlock()

	aliveCells := calculateAliveCells(p,  makeImmutableMatrix(*g.World))
	final := FinalTurnComplete{CompletedTurns: g.Turns, Alive: aliveCells}

	c.events <- final //sending event down events channel
	sendWriteCommand(p, c, g.Turns, *g.World)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{g.Turns, Quitting}
	done <- true
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
