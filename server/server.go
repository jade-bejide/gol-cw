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

		if isAlive(x, u, world) { liveNeighbours += 1}
		if isAlive(x, d, world) { liveNeighbours += 1}
		if isAlive(l, u, world) { liveNeighbours += 1}
		if isAlive(r, u, world) { liveNeighbours += 1}
		if isAlive(l, d, world) { liveNeighbours += 1}
		if isAlive(r, d, world) { liveNeighbours += 1}
		if isAlive(l, y, world) { liveNeighbours += 1}
		if isAlive(r, y, world) { liveNeighbours += 1}

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
	//fmt.Println(nextWorld)
	return nextWorld
}

func takeTurns(g *Gol){
	g.Turn = 0
	g.TurnMut.Lock()
	for g.Turn < g.Params.Turns {
		g.TurnMut.Unlock()
		g.WorldMut.Lock() //block if we're reading the current alive cells
		g.World = calculateNextState(g.Params, /*_,*/ g.World, 0, g.Params.ImageHeight, g.Turn)
		g.Turn++
		g.WorldMut.Unlock() //allow us to report the alive cells on the following turn (once we're done here)
		g.TurnMut.Lock()
		//c.events <- TurnComplete{turn}
	}
	g.TurnMut.Unlock()

	//done <- true
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
	TurnMut sync.Mutex
}

func (g *Gol) TakeTurns(req stubs.Request, res *stubs.Response) (err error){

	g.Params = stubs.Params(req.Params)

	g.World = req.World
	g.Turn = 0

	//done := make(chan bool)
	takeTurns(g)
	//<-done



	res.World = g.World
	res.Turns = g.Turn
	res.Alive = calculateAliveCells(g.Params, g.World)
	return
}

//before a client closes, it calls the server to reset the world
func (g *Gol) ResetWorld(req stubs.CloseRequest, res *stubs.CloseResponse) (err error) {
	fmt.Println("Closing client")
	if req.Close == true {
		g.World = make([][]uint8, 0)
		g.Turn = 0
		res.ResponseCode = 0
	} else { res.ResponseCode = -1 }


	return
}


func (g *Gol) ReportAlive(req stubs.AliveRequest, res *stubs.AliveResponse) (err error){

	g.TurnMut.Lock()
	g.WorldMut.Lock()


	res.Alive = len(calculateAliveCells(g.Params, g.World))
	res.OnTurn = g.Turn
	fmt.Println(res.Alive, res.OnTurn)
	g.TurnMut.Unlock()
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