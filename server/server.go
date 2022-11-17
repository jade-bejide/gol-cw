package main

import (
	"flag"
	_ "flag"
	"fmt"
	//"fmt"
	_ "math/rand"
	"net"
	_ "net"
	"net/rpc"
	_ "net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var kill = make(chan bool, 1)
var runningCalls = sync.WaitGroup{}

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

	return nextWorld
}

func takeTurns(g *Gol){
	g.TurnMut.Lock()
	g.setTurn(0)
	for g.Turn < g.Params.Turns {
		select{
			case <-g.Done:
				g.TurnMut.Unlock()
				return
			default:
        		g.TurnMut.Unlock()
				g.WorldMut.Lock() //block if we're reading the current alive cells
				g.World = calculateNextState(g.Params, /*_,*/ g.World, 0, g.Params.ImageHeight, g.Turn)
				g.setTurn(g.Turn + 1)
				g.WorldMut.Unlock() //allow us to report the alive cells on the following turn (once we're done here)
        		g.TurnMut.Lock()
				//c.events <- TurnComplete{turn}
				// fmt.Println("im on turn ", g.Turn)
		}

	}
	g.TurnMut.Unlock()
	return
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

func resetGol(g *Gol){

	//g.WorldMut.Lock()
	//g.TurnMut.Lock()
	//
	//g.Params = stubs.Params{}
	//g.World = make([][]uint8, 0)
	//g.Turn = 0
	//g.Done = make(chan bool, 1)
	//
	//g.TurnMut.Unlock()
	//g.WorldMut.Unlock()

	g.setParams(stubs.Params{})
	g.setWorld(make([][]uint8, 0))
	g.setTurn(0)
	g.setDone(make(chan bool, 1))
}

type Gol struct {
	Mut sync.Mutex
	WorldMut sync.Mutex
	TurnMut sync.Mutex
	Params stubs.Params
	World [][]uint8
	Turn int
	Done chan bool
}

//internal methods (safe setters)
func (g *Gol) setParams(p stubs.Params){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Params = p
}

func (g *Gol) setWorld(w [][]uint8){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.World = w
}

func (g *Gol) setTurn(t int){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Turn = t
}

func (g *Gol) setDone(d chan bool){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Done = d
}

//RPC methods
func (g *Gol) TakeTurns(req stubs.Request, res *stubs.Response) (err error){
	runningCalls.Add(1); defer runningCalls.Done()
	// fmt.Println("started TakeTurns()")

	resetGol(g)
	g.setParams(stubs.Params(req.Params))
	g.setWorld(req.World)

	takeTurns(g)

	g.WorldMut.Lock()
	g.TurnMut.Lock()
	res.World = g.World
	res.Turn = g.Turn
	res.Alive = calculateAliveCells(g.Params, g.World)
	g.TurnMut.Unlock()
	g.WorldMut.Unlock()

	// fmt.Println("stopped TakeTurns()")
	return
}


func (g *Gol) ReportAlive(req stubs.EmptyRequest, res *stubs.AliveResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()
	// fmt.Println("started ReportAlive()")

	g.WorldMut.Lock()
	g.TurnMut.Lock()
	res.Alive = len(calculateAliveCells(g.Params, g.World))
	res.OnTurn = g.Turn
	// fmt.Println(res.Alive, res.OnTurn)
	g.TurnMut.Unlock()
	g.WorldMut.Unlock()

	// fmt.Println("stopped ReportAlive()")
	return
}

func (g *Gol) PollWorld(req stubs.EmptyRequest, res *stubs.Response) (err error){
	runningCalls.Add(1); defer runningCalls.Done()
	// fmt.Println("started PollWorld()")

	g.WorldMut.Lock()
	g.TurnMut.Lock()
	res.World = g.World
	res.Turn = g.Turn
	res.Alive = calculateAliveCells(g.Params, g.World)
	g.TurnMut.Unlock()
	g.WorldMut.Unlock()
	//// fmt.Println("I am responding with the world on turn", res.Turn)
	//fmt.Printf("The world looks like")
	//// fmt.Println(res.World)

	// fmt.Println("stopped PollWorld()")
	return
}

//asks the only looping rpc call to finish when ready (takeTurns())
func (g *Gol) Finish(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()
	// fmt.Println("started Finish()")

	g.Mut.Lock()
	g.Done <- true
	g.Mut.Unlock()

	// fmt.Println("stopped Finish()")
	return
}

// lets the server know that it needs to shut down as soon as possible
// returns the number of currently running rpc calls by reading the value of the waitgroup (will always return at least 1, since it includes itself)
func (g *Gol) Kill(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()
	// fmt.Println("started Kill()")

	kill <- true
	// fmt.Println("server set to close when ready")
	// fmt.Println("stopped Kill()")
	return
}

func runServer(s *rpc.Server, l *net.Listener){
	go s.Accept(*l)
	<-kill
	// fmt.Println("closed acceptor")
	return
}

func main() {
	portPtr := flag.String("port", "8030", "port used; default: 8030")
	flag.Parse()
	//rand.Seed(time.Now().UnixNano())
	server := rpc.NewServer()
	err := server.Register(&Gol{})
	if err != nil {
		fmt.Printf("Error registering new rpc server with Gol struct; %s\n", err)
	}
	listener, err := net.Listen("tcp", ":"+*portPtr)
	if(err != nil) { panic(err) }
	// fmt.Println("server listening on port "+*portPtr)

	runServer(server, &listener)

	// fmt.Println("server waiting for all calls to terminate")
	runningCalls.Wait()
	// fmt.Println("all calls terminated")

	//try to close the server
	err = listener.Close()
	if err != nil {
		fmt.Printf("Error trying to use/Close() listener; %s\n", err)
	}

}