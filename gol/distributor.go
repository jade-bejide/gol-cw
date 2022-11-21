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

//type Boolean struct {
//	B bool
//
//}

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
	WorldA [][]uint8
	WorldB [][]uint8
	World *[][]uint8
	Next *[][]uint8
	WorldMut sync.Mutex
	WorkGroup sync.WaitGroup
}

func (g *Gol) swapWorld() {
	g.WorldMut.Lock()
	if g.World == &g.WorldA {
		g.World = &g.WorldB
		g.Next = &g.WorldA
	}else{
		g.World = &g.WorldA
		g.Next = &g.WorldB
	}
	g.WorldMut.Unlock()
}

func newGol(width, height int) Gol{
	worldA := genWorldBlock(width, height)
	worldB := genWorldBlock(width, height)
	return Gol{Turns: 0, TurnsMut: sync.Mutex{}, WorldA: worldA, WorldB: worldB, World: &worldA, Next: &worldB, WorldMut: sync.Mutex{}};
}

//returns a closure of a 2d array of uint8s
func makeImmutableMatrix(m [][]uint8) func(x, y int) uint8 {
	return func(x, y int) uint8 {
		return m[y][x]
	}
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

type WorldBlock struct {
	Data  [][]byte
	Index int
}

//completes one turn of gol
func (g *Gol) calculateNextState(p Params, c distributorChannels, y1 int, y2 int) {
	x := 0

	height := y2 - y1

	for x < p.ImageWidth {
		j := y1
		for y := 0; y < height; y++ {
			neighbours := countLiveNeighbours(p, x, j, *g.World) //reading needs no mutex
			alive := isAlive(x, j, *g.World)

			alive = updateState(alive, neighbours)

			if alive {
				(*g.Next)[y][x] = 255 //writing needs no mutex
			} else {
				(*g.Next)[y][x] = 0
			}
			if (*g.World)[j][x] != (*g.Next)[y][x] {
				cell := util.Cell{X: x, Y: j}
				c.events <- CellFlipped{CompletedTurns: g.Turns, Cell: cell}
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

func (g *Gol) worker(p Params, c distributorChannels, y1 int, y2 int, workerId int) {
	//do the things
	g.calculateNextState(p, c, y1, y2)
	g.WorkGroup.Done()
	//outCh <- WorldBlock{Index: workerId, Data: nextWorld}
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
			events <- AliveCellsCount{g.Turns, len(calculateAliveCells(p, *g.World))}
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
			fmt.Println("P")
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
			c.events <- FinalTurnComplete{CompletedTurns: g.Turns, Alive: calculateAliveCells(p, *g.World)}
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
		}
	}

	splits := spreadWorkload(len(world), p.Threads)
	fmt.Println(splits)

	//outCh := make(chan WorldBlock)
	// TODO: Execute all turns of the Game of Life.

	//ticker tools
	//sharedTurns := Turns{0, sync.Mutex{}}
	//sharedWorld := SharedWorld{world, sync.Mutex{}}
	g := newGol(p.ImageWidth, p.ImageHeight)

	pauseLock := sync.Mutex{}
	done = make(chan bool)
	go g.ticks(p, c.events, aliveCellsPollDelay)
	go g.handleSDL(p, c, keyPresses, &pauseLock)

	//outCh := make(chan )

	g.TurnsMut.Lock()

	for g.Turns = 0; g.Turns < p.Turns; g.Turns++ {
		//pauseLock.Lock()
		g.TurnsMut.Unlock()
		g.WorkGroup.Add(p.Threads)
		for i := 0; i < p.Threads; i++ {
			go g.worker(p, c, splits[i], splits[i+1], i)
		}
		g.WorkGroup.Wait()
		fmt.Printf("TURN %d\n", g.Turns)

		//nextWorld := make([][][]byte, p.Threads)

		//g.WorldMut.Lock()
		//for i := 0; i < p.Threads; i++ {
		//	section := <-outCh
		//	//nextWorld[section.Index] = section.Data
		//	for j := splits[section.Index]; j < splits[section.Index + 1]; j++ { //(re)sets each row
		//		(*g.Next)[j] = section.Data[j]
		//	}
		//	fmt.Printf("collected %d\n", section.Index)
		//}
		//g.WorldMut.Unlock()
		//fmt.Println(g.Next)

		//g.WorldMut.Lock()
		//world = make([][]byte, 0)
		//for _, section := range nextWorld {
		//	for _, row := range section {
		//		world = append(world, row)
		//	}
		//}
		//g.World = world
		//g.WorldMut.Unlock()

		g.TurnsMut.Lock()
		c.events <- TurnComplete{g.Turns}
		g.swapWorld()
		//pauseLock.Unlock()
	}

	g.TurnsMut.Unlock()
	// TODO: Report the final state using FinalTurnCompleteEvent.

	aliveCells := calculateAliveCells(p, *g.World)
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
