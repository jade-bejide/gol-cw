package main

import (
	"flag"
	_ "flag"
	"fmt"
	_ "math/rand"
	"net"
	_ "net"
	"net/rpc"
	_ "net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// helpers

func updateState(isAlive bool, neighbours int) bool {
	return isAlive && neighbours > 1 && neighbours < 4 || !isAlive && neighbours == 3
}

func isAlive(x int, y int, world [][]byte) bool {
	return world[y][x] != 0
}

//creates a 2D slice of a world of size height x width
func genWorldBlock(height int, width int) [][]byte {
	worldBlock := make([][]byte, height)

	for i := range worldBlock {
		worldBlock[i] = make([]byte, width)
	}

	return worldBlock
}


// logic engine

func countLiveNeighbours(p stubs.Params, x int, y int, world [][]byte) int {
		liveNeighbours := 0

		w := p.ImageWidth - 1
		h := p.ImageHeight - 1

		l := x - 1
		r := x + 1
		u := y + 1
		d := y - 1

		if l < 0 {l = w}
		if r > w {r = 0}
		if u > h {u = 0}
		if d < 0 {d = h}

		if isAlive(u, x, world) { liveNeighbours += 1}
		if isAlive(d, x, world) { liveNeighbours += 1}
		if isAlive(u, l, world) { liveNeighbours += 1}
		if isAlive(u, r, world) { liveNeighbours += 1}
		if isAlive(d, l, world) { liveNeighbours += 1}
		if isAlive(d, r, world) { liveNeighbours += 1}
		if isAlive(y, l, world) { liveNeighbours += 1}
		if isAlive(y, r, world) { liveNeighbours += 1}

		return liveNeighbours
	}

func calculateNextState(p stubs.Params, /*c distributorChannels, */world [][]byte, y1 int, y2 int, turn int) [][]byte {
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
				//cell := util.Cell{X: x, Y: j}
				//c.events <- CellFlipped{CompletedTurns: turn, Cell: cell}
			}

			j += 1
		}
		x += 1
	}
	fmt.Println(nextWorld)
	return nextWorld
}

func takeTurns(g *Gol){
	g.Turn = 0
	for g.Turn < g.Params.Turns {
		g.WorldMut.Lock() //block if we're reading the current alive cells
		g.World = calculateNextState(g.Params, /*_,*/ g.World, 0, g.Params.ImageHeight, g.Turn)
		g.Turn++
		g.WorldMut.Unlock() //allow us to report the alive cells on the following turn (once we're done here)
		//c.events <- TurnComplete{turn}
	}

	fmt.Println("In Take Turns")
	for _, row := range *world {
		fmt.Println(row)
	}

	fmt.Println()
}

func calculateAliveCells(p stubs.Params, world [][]byte) []util.Cell {
	var cells []util.Cell

	for x := 0; x < p.ImageWidth; x++{
		for y := 0; y < p.ImageHeight; y++ {
			if isAlive(x, y, world) {
				c := util.Cell{x, y}
				cells = append(cells, c)
			}
		}
	}

	return cells
}

type Gol struct {
	Params stubs.Params
	World [][]uint8
	WorldMut sync.Mutex
	Turn int
}

func (g *Gol) TakeTurns(req stubs.Request, res *stubs.Response) (err error){
	g.Params = stubs.Params(req.Params)

	g.World = req.World
	g.Turn = 0

	fmt.Println("Before")
	for _, row := range g.World {
		fmt.Println(row)
	}

	fmt.Println()
	fmt.Println(g.Params.Turns, g.Params.ImageWidth, g.Params.Threads)
	takeTurns(g)



	res.World = g.World
	res.Turns = g.Turn
	res.Alive = calculateAliveCells(g.Params, g.World)
	return
}

func (g *Gol) AliveHandler(req stubs.AliveRequest, res *stubs.AliveResponse) (err error){
	fmt.Println("AliveHandler called remotely")
	g.WorldMut.Lock()
	res.Alive = len(calculateAliveCells(g.Params, g.World))
	res.OnTurn = g.Turn
	g.WorldMut.Unlock()


	return
}

func main() {
	portPtr := flag.String("port", "8030", "port used; default: 8030")
	flag.Parse()
	//rand.Seed(time.Now().UnixNano())
	rpc.Register(&Gol{})
	listener, err := net.Listen("tcp", ":"+*portPtr)
	if(err != nil) { panic(err) }
	defer listener.Close()
	rpc.Accept(listener)
}