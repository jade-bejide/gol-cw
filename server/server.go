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

type Active struct {
	Top int
	Bottom int
}

var kill = make(chan bool, 1)
var runningCalls = sync.WaitGroup{}
var active = Active{}

// helpers

func updateState(isAlive bool, neighbours int) bool {
	return isAlive && neighbours > 1 && neighbours < 4 || !isAlive && neighbours == 3
}

func isAlive(x int, y int, world func(x, y int) uint8) bool {
	return world(x, y) != 0
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

func countLiveNeighbours(p stubs.Params, x int, y int, worldReadOnly func(x, y int) uint8) int {
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

		if isAlive(x, u, worldReadOnly) { liveNeighbours += 1}
		if isAlive(x, d, worldReadOnly) { liveNeighbours += 1}
		if isAlive(l, u, worldReadOnly) { liveNeighbours += 1}
		if isAlive(r, u, worldReadOnly) { liveNeighbours += 1}
		if isAlive(l, d, worldReadOnly) { liveNeighbours += 1}
		if isAlive(r, d, worldReadOnly) { liveNeighbours += 1}
		if isAlive(l, y, worldReadOnly) { liveNeighbours += 1}
		if isAlive(r, y, worldReadOnly) { liveNeighbours += 1}

		return liveNeighbours
	}

func calculateNextStateHalo(g *Gol, p stubs.Params, worldReadOnly func(x, y int) uint8, y1 int, y2 int, turn int) {

	height := y2 - y1

	g.Mut.Lock(); defer g.Mut.Unlock()
	for x := 0; x < p.ImageWidth; x++ {
		//pad one as to ignore the top halo
		for y := y1; y <= height; y++ {
			yWorld := y + y1
			neighbours := countLiveNeighbours(p, x, yWorld, worldReadOnly)
			alive := isAlive(x, yWorld, worldReadOnly)
			alive = updateState(alive, neighbours)

			if alive {
				g.Slice[y][x] = 255
			} else {
				g.Slice[y][x] = 0
			}
		}
	}
}

//func calculateNextState(g *Gol, p stubs.Params, /*c distributorChannels, */world [][]byte, y1 int, y2 int, turn int) {
//
//	height := y2 - y1
//
//	g.Mut.Lock(); defer g.Mut.Unlock()
//	for x := 0; x < p.ImageWidth; x++ {
//		for y := 0; y < height; y++ {
//			yWorld := y + y1
//			neighbours := countLiveNeighbours(p, x, yWorld, world)
//			alive := isAlive(x, yWorld, world)
//			alive = updateState(alive, neighbours)
//
//			if alive {
//				g.Strip[y][x] = 255
//			} else {
//				g.Strip[y][x] = 0
//			}
//		}
//	}
//}

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

func (g *Gol) calculateAliveCells(p stubs.Params, worldReadOnly func(x, y int) uint8) []util.Cell {
	var cells []util.Cell

	for x := 0; x < p.ImageWidth; x++{
		for y := active.Top; y < active.Bottom; y++ {
			if isAlive(x, y, worldReadOnly) {
				c := util.Cell{x, y}
				cells = append(cells, c)
			}
		}
	}

	return cells
}

func (g *Gol) aliveStrip(worldReadOnly func (x, y int) uint8) []util.Cell {
	var cells []util.Cell

	height := len(g.Slice) - 2
	for x := 0; x < g.Params.ImageWidth; x++ {
		for y := 0; y < height; y++ {
			if isAlive(x, y, worldReadOnly) {
				c := util.Cell{x, y+active.Top}
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
	//g.setWorld(make([][]uint8, 0))
	g.setTurn(0)
	g.setDone(make(chan bool, 1))
}

type Gol struct {
	Mut sync.Mutex
	WorldMut sync.Mutex
	TurnMut sync.Mutex

	Params stubs.Params
	ID int

	SliceMut sync.Mutex //will need locking on access to slice, OR top/bottom halo (reference tie-ins)
	Slice [][]uint8 //active part of the slice is all but the first and last row
	ReadOnlySlice func (x, y int) uint8
	TopHalo []uint8 //refers to first row of slice
	BottomHalo []uint8 //refers to last row of slice

	WorkerAbove *rpc.Client
	WorkerBelow *rpc.Client
	WaitForReadMut sync.Mutex

	Turn int
	Done chan bool
}



//internal methods (safe setters)
func (g *Gol) setParams(p stubs.Params){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Params = p
}

//func (g *Gol) setWorld(w [][]uint8){
//	g.Mut.Lock(); defer g.Mut.Unlock()
//	g.World = w
//}

func (g *Gol) setTurn(t int){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Turn = t
}

func (g *Gol) setDone(d chan bool){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Done = d
}

func (g *Gol) setSlice(s [][]uint8){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.Slice = s
}

func (g *Gol) setID(id int){
	g.Mut.Lock(); defer g.Mut.Unlock()
	g.ID = id
}

//func (g *Gol) setStrip() (err error){ //depends entirely on slice, this means it can return errors
//	g.Mut.Lock(); defer g.Mut.Unlock()
//	if g.Slice.To == 0 && g.Slice.From == 0 {
//		return errors.New("Slice is nil")
//	}
//	if g.Params.ImageWidth == 0 {
//		return errors.New("Params is nil")
//	}
//
//	subStrip := make([][]uint8, g.Slice.To - g.Slice.From)
//	for i := range subStrip {
//		subStrip[i] = make([]uint8, g.Params.ImageWidth)
//	}
//
//	g.Strip = subStrip
//	return
//}

func (g *Gol) connectWorkers(above, below string) (err error) {
	g.WorkerAbove, err = rpc.Dial("tcp", above)
	g.WorkerBelow, err = rpc.Dial("tcp", below)
	return
}

func (g *Gol) Setup(req stubs.SetupRequest, res *stubs.SetupResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()

	fmt.Println("Setting up")

	resetGol(g)
	g.setID(req.ID)
	// fmt.Println(g.ID, req.ID)
	// the below line needs to include the ghost rows from the getgo
	g.setSlice(append(req.Slice, make([][]uint8, 2)...)) //is the right size off the bat, since we add two extra rows as ghost
	g.ReadOnlySlice = func(x, y int) uint8{
		return g.Slice[y][x]
	}
	g.TopHalo = g.Slice[0]
	g.BottomHalo = g.Slice[len(g.Slice) - 1] //by reference, so we need the mutex
	g.setParams(req.Params)

	active = Active{ Top: 1, Bottom: len(g.Slice) - 2 } //top and bottom index of the part we write to

	err = g.connectWorkers(req.Above, req.Below)
	// fmt.Println(g.ID, g.Params, g.Slice)

	res.ID = req.ID
	res.Success = err == nil
	return
}

////Joins halos and strip together to get a world
//func (g *Gol) combineWorld(topHalo []byte, bottomHalo []byte) {
//	g.Mut.Lock(); defer g.Mut.Unlock()
//	g.World = make([][]uint8, 0)
//
//	g.World = append(g.World, topHalo)
//
//	for _, row := range g.Strip {
//		g.World = append(g.World, row)
//	}
//
//	g.World = append(g.World, bottomHalo)
//}

//func (g *Gol) HaloSetup(req stubs.HaloSetupRequest, res *stubs.SetupResponse) (err error) {
//	runningCalls.Add(1); defer runningCalls.Done()
//
//	resetGol(g)
//	g.setID(req.ID)
//	g.setSlice(req.Slice)
//	g.setParams(req.Params)
//	err = g.setStrip()
//
//	if err != nil { return err }
//
//	g.combineWorld(req.TopHalo, req.BottomHalo) //generate subworld from strip and halos
//	res.Slice = req.Slice
//
//	return
//}
////
//func (g *Gol) HaloTakeTurn(req stubs.HaloRequest, res *stubs.Response) (err error) {
//	runningCalls.Add(1); defer runningCalls.Done()
//
//
//		//g.combineWorld(req.TopHalo, req.BottomHalo)
//		excludeTopRow := 1
//		excluseBotRow := len(g.Slice) - 1
//
//		worldReadOnly := func(x, y int) uint8{
//			return g.Slice[y][x]
//		}
//
//		calculateNextStateHalo(g, g.Params, /*_,*/ worldReadOnly, excludeTopRow, excluseBotRow, g.Turn)
//
//		g.TurnMut.Lock()
//		g.setTurn(g.Turn + 1)
//		g.TurnMut.Unlock()
//
//		g.Mut.Lock()
//		res.ID = g.ID
//		res.Strip = g.Strip
//		res.Slice = g.Slice
//		res.Turn = g.Turn
//		res.Alive = g.aliveStrip()
//
//		g.Mut.Unlock()
//
//	return
//}

func (g *Gol) lockOnParity() {
	if(g.ID % 2 == 0){
		g.WaitForReadMut.Lock() //taketurns will block on this lock until it has been read from, if it is even
	}
	return
}

func (g *Gol) GetHaloRow(req stubs.HaloRequest, res *stubs.HaloResponse) (err error) {
	if(g.ID % 2 == 0){
		defer g.WaitForReadMut.Unlock()
	}
	if req.Top {
		res.Halo = g.TopHalo
	}else{
		res.Halo = g.BottomHalo
	}

	return
}

//RPC methods
func (g *Gol) TakeTurns(req stubs.Request, res *stubs.Response) (err error){
	runningCalls.Add(1); defer runningCalls.Done()

	//fmt.Println(g)

	for i := 0; i < req.Params.Turns; i++ {
		//g.setWorld(req.World)
		excludeTopRow := 1
		excluseBotRow := len(g.Slice) - 1

		g.SliceMut.Lock()
		calculateNextStateHalo(g, g.Params, g.ReadOnlySlice, excludeTopRow, excluseBotRow, g.Turn)
		g.SliceMut.Unlock()

		g.TurnMut.Lock()
		g.setTurn(g.Turn + 1)
		g.TurnMut.Unlock()

		g.lockOnParity() //locks the following lock if even
		g.WaitForReadMut.Lock()

		//then we read from others
		reqAbove := stubs.HaloRequest{Top: false}//want the first processed row, not the outdated halo
		resAbove := new(stubs.HaloResponse)
		reqBelow := stubs.HaloRequest{Top: true} //want the first processed row, not the outdated halo
		resBelow := new(stubs.HaloResponse)
		err = g.WorkerAbove.Call(stubs.GetHaloHandler, reqAbove, resAbove)
		if err != nil { panic(err) }
		err = g.WorkerBelow.Call(stubs.GetHaloHandler, reqBelow, resBelow)
		if err != nil {
			panic(err)
		}

		g.WaitForReadMut.Unlock()

		//write the new data into our slice
		g.SliceMut.Lock()

		g.TopHalo = resAbove.Halo
		g.BottomHalo = resBelow.Halo

		g.SliceMut.Unlock()
	}


	g.Mut.Lock()

	res.Slice = g.Slice[1:len(g.Slice) - 1]
	res.Turn = g.Turn
	res.Alive = g.aliveStrip(g.ReadOnlySlice)

	g.Mut.Unlock()

	fmt.Println("IM ALL DONE!!")

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
		res.Alive = g.calculateAliveCells(g.Params, g.ReadOnlySlice)
	} else {
		res.Alive = g.aliveStrip(g.ReadOnlySlice)
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