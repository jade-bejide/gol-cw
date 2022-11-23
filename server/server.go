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

func calculateNextState(g *Gol, p stubs.Params, /*c distributorChannels, */world [][]byte, y1 int, y2 int, turn int) {

	height := y2 - y1

	g.Mut.Lock(); defer g.Mut.Unlock()
	for x := 0; x < p.ImageWidth; x++ {
		for y := 0; y < height; y++ {
			yWorld := y + y1
			neighbours := countLiveNeighbours(p, x, yWorld, world)
			alive := isAlive(x, yWorld, world)
			alive = updateState(alive, neighbours)

			if alive {
				g.Strip[y][x] = 255
			} else {
				g.Strip[y][x] = 0
			}
		}
	}
}

//func takeTurns(g *Gol){
//	g.TurnMut.Lock()
//
//	g.setTurn(0)
//
//	for g.Turn < g.Params.Turns {
//		select{
//			case <-g.Done:
//				g.TurnMut.Unlock()
//				return
//			default:
//        		g.TurnMut.Unlock()
//				g.WorldMut.Lock() //block if we're reading the current alive cells
//				g.World = calculateNextState(g.Params, /*_,*/ g.World, 0, g.Params.ImageHeight, g.Turn)
//				g.setTurn(g.Turn + 1)
//				g.WorldMut.Unlock() //allow us to report the alive cells on the following turn (once we're done here)
//        		g.TurnMut.Lock()
//				//c.events <- TurnComplete{turn}
//		}
//
//	}
//	g.TurnMut.Unlock()
//	return
//}

func (g *Gol) calculateAliveCells(p stubs.Params, world [][]byte) []util.Cell {
	var cells []util.Cell

	for x := 0; x < p.ImageWidth; x++{
		for y := g.Slice.From; y < g.Slice.To; y++ {
			if isAlive(x, y, world) {
				c := util.Cell{x, y}
				cells = append(cells, c)
			}
		}
	}

	return cells
}

func (g *Gol) aliveStrip() []util.Cell {
	var cells []util.Cell

	height := g.Slice.To - g.Slice.From
	for x := 0; x < g.Params.ImageWidth; x++ {
		for y := 0; y < height; y++ {
			if isAlive(x, y, g.Strip) {
				c := util.Cell{x, y+g.Slice.From}
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
	Slice stubs.Slice
	ID int

	World [][]uint8
	Strip [][]uint8

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

func (g *Gol) setSlice(s stubs.Slice){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Slice = s
}

func (g *Gol) setID(id int){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.ID = id
}

func (g *Gol) setStrip() (err error){ //depends entirely on slice, this means it can return errors
	g.Mut.Lock(); defer g.Mut.Unlock()
	if g.Slice.To == 0 && g.Slice.From == 0 {
		fmt.Println("Slice is nil")
	}
	if g.Params.ImageWidth == 0 {
		fmt.Println("Params is nil")
	}

	subStrip := make([][]uint8, g.Slice.To - g.Slice.From)
	for i := range subStrip {
		subStrip[i] = make([]uint8, g.Params.ImageWidth)
	}

	g.Strip = subStrip
	return
}

func (g *Gol) Setup(req stubs.SetupRequest, res *stubs.SetupResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()

	fmt.Println("Setting up")

	resetGol(g)
	g.setID(req.ID)
	fmt.Println(g.ID, req.ID)
	g.setSlice(req.Slice)
	g.setParams(req.Params)
	g.setWorld(req.World)
	err = g.setStrip()
	res.Slice = req.Slice

	fmt.Println(g.ID, g.Params, g.Slice)

	return err
}

//RPC methods
func (g *Gol) TakeTurn(req stubs.Request, res *stubs.Response) (err error){
	runningCalls.Add(1); defer runningCalls.Done()
	//
	//fmt.Println("Taking turn")

	g.setWorld(req.World)
	calculateNextState(g, g.Params, /*_,*/ g.World, g.Slice.From, g.Slice.To, g.Turn)

	g.TurnMut.Lock() //we lock on read to avoid stale values and race conditions
	g.setTurn(g.Turn + 1)
	g.TurnMut.Unlock()

	g.Mut.Lock()
	res.ID = g.ID
	fmt.Println("Returning", res.ID)
	res.Strip = g.Strip
	res.Slice = g.Slice
	res.Turn = g.Turn
	res.Alive = g.aliveStrip()

	g.Mut.Unlock()

	return
}

//func (g *Gol) PauseGol(req stubs.PauseRequest, res *stubs.PauseResponse) (err error) {
//	runningCalls.Add(1); defer runningCalls.Done()
//	if req.Pause {
//	    g.WorldMut.Lock()
//	    g.TurnMut.Lock()
//	    res.World = g.World
//	    res.Turns = g.Turn
//	} else {
//	    res.Turns = g.Turn
//	    g.WorldMut.Unlock()
//	    g.TurnMut.Unlock()
//	 }
//	return
//}


func (g *Gol) ReportAlive(req stubs.EmptyRequest, res *stubs.AliveResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()

	g.WorldMut.Lock()
	g.TurnMut.Lock()
	if g.Params.Turns == 0 {
		res.Alive = g.calculateAliveCells(g.Params, g.World)
	} else {
		res.Alive = g.aliveStrip()
	}
	res.OnTurn = g.Turn
	g.TurnMut.Unlock()
	g.WorldMut.Unlock()

	return
}

//func (g *Gol) PollWorld(req stubs.EmptyRequest, res *stubs.Response) (err error){
//	runningCalls.Add(1); defer runningCalls.Done()
//
//	g.WorldMut.Lock()
//	g.TurnMut.Lock()
//	res.World = g.World
//	res.Turn = g.Turn
//	res.Alive = calculateAliveCells(g.Params, g.World)
//	g.TurnMut.Unlock()
//	g.WorldMut.Unlock()
//	//fmt.Println("I am responding with the world on turn", res.Turn)
//	//fmt.Printf("The world looks like")
//	//fmt.Println(res.World)
//
//	return
//}

//asks the only looping rpc call to finish when ready (takeTurns())
func (g *Gol) Finish(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()

	g.Mut.Lock()
	g.Done <- true
	g.Mut.Unlock()

	return
}

// lets the server know that it needs to shut down as soon as possible
// returns the number of currently running rpc calls by reading the value of the waitgroup (will always return at least 1, since it includes itself)
func (g *Gol) Kill(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()

	kill <- true
	fmt.Println("server set to close when ready")
	return
}

func runServer(s *rpc.Server, l *net.Listener){
	go s.Accept(*l)
	<-kill
	fmt.Println("closed acceptor")
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
	fmt.Println("server listening on port "+*portPtr)

	runServer(server, &listener)

	fmt.Println("server waiting for all calls to terminate")
	runningCalls.Wait()
	fmt.Println("all calls terminated")

	//try to close the server
	err = listener.Close()
	if err != nil {
		fmt.Printf("Error trying to use/Close() listener; %s\n", err)
	}

}