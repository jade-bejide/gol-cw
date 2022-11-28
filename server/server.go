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

//indices of where the active slice sits, and where it is in the image
type Active struct {
	Top         int //will always be 1
	Bottom      int //length of slice - 2
	AliveOffset int //how far down the whole image this workers slice is
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
	//h := p.ImageHeight - 1 //doesnt wrap on height with halos

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
	//if u > h {u = 0} //doesnt wrap on height with halos
	//if d < 0 {d = h}

	if isAlive(x, u, worldReadOnly) {
		liveNeighbours += 1
	}
	if isAlive(x, d, worldReadOnly) {
		liveNeighbours += 1
	}
	if isAlive(l, u, worldReadOnly) {
		liveNeighbours += 1
	}
	if isAlive(r, u, worldReadOnly) {
		liveNeighbours += 1
	}
	if isAlive(l, d, worldReadOnly) {
		liveNeighbours += 1
	}
	if isAlive(r, d, worldReadOnly) {
		liveNeighbours += 1
	}
	if isAlive(l, y, worldReadOnly) {
		liveNeighbours += 1
	}
	if isAlive(r, y, worldReadOnly) {
		liveNeighbours += 1
	}

	return liveNeighbours
}

func calculateNextStateHalo(g *Gol, p stubs.Params, worldReadOnly func(x, y int) uint8) {

	g.Mut.Lock()
	defer g.Mut.Unlock()
	for x := 0; x < p.ImageWidth; x++ {
		//fmt.Println("Column", x)
		for y := active.Top; y <= active.Bottom; y++ { //inclusive by our definition of Active.Bottom
			neighbours := countLiveNeighbours(p, x, y, worldReadOnly)
			alive := isAlive(x, y, worldReadOnly)
			alive = updateState(alive, neighbours)

			if alive {
				g.Slice.Write[y][x] = 255
			} else {
				g.Slice.Write[y][x] = 0
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

	for x := 0; x < p.ImageWidth; x++ {
		for y := active.Top; y < active.Bottom; y++ {
			if isAlive(x, y, worldReadOnly) {
				c := util.Cell{x, y}
				cells = append(cells, c)
			}
		}
	}

	return cells
}

func (g *Gol) aliveStrip(worldReadOnly func(x, y int) uint8) []util.Cell {
	var cells []util.Cell

	height := active.Bottom - active.Top + 1 // must add one as the indices are inclusive
	for x := 0; x < g.Params.ImageWidth; x++ {
		for y := active.Top; y <= height; y++ { //height needs to be inclusive as its defined using the index
			if isAlive(x, y, worldReadOnly) {
				c := util.Cell{x, y + active.AliveOffset} //for objective reporting, we need to add the offset
				cells = append(cells, c)
			}
		}
	}

	return cells
}

func resetGol(g *Gol) {

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

//set each element in dst to that of src, must be equal size, non-zero length and rectangular
func copyEqualSizeSlice(src, dst [][]uint8) {
	h := len(src)
	w := len(src[0])
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst[y][x] = src[y][x]
		}
	}
}

type SwapSlice struct {
	Read  [][]uint8
	Write [][]uint8 //Write pertains only to writing during GOL logic, other times Read is written to and read from
}

func NewSwapSlice(g *Gol, s [][]uint8) SwapSlice {
	write := make([][]uint8, len(s)-2)
	for i, _ := range write {
		write[i] = make([]uint8, len(s[0]))
	}

	ss := SwapSlice{
		Read:  s,
		Write: append(append([][]uint8{g.TopHalo}, write...), g.BottomHalo), //wraps new world in halos
	}
	return ss
}
func (s *SwapSlice) setReadToWrite(g *Gol) {
	copyEqualSizeSlice(s.Write, s.Read)
	g.TopHalo = s.Read[0]
	g.BottomHalo = s.Read[active.Bottom+1]
}

type Gol struct {
	Mut      sync.Mutex
	WorldMut sync.Mutex
	TurnMut  sync.Mutex

	Params   stubs.Params
	ID       int
	IsIDEven bool

	SliceMut sync.Mutex //will need locking on access to slice, OR top/bottom halo (reference tie-ins)
	//Slice [][]uint8 //active part of the slice is all but the first and last row
	Slice         SwapSlice
	ReadOnlySlice func(x, y int) uint8
	TopHalo       []uint8 //refers to first row of slice
	BottomHalo    []uint8 //refers to last row of slice

	//for turn advertisation, they must be 0 buffered else it will allow the worker to take turns ahead of their co-workers
	TopHalosCh    chan []uint8
	BottomHalosCh chan []uint8

	WorkerAbove *rpc.Client
	WorkerBelow *rpc.Client
	IsAboveEven bool
	IsBelowEven bool
	//WaitForReadCh chan bool //if there is something in this channel, its safe to read other nodes (dont block)

	Turn         int
	Done         chan bool
}

//internal methods (safe setters)
func (g *Gol) setParams(p stubs.Params) {
	g.Mut.Lock()
	defer g.Mut.Unlock()
	g.Params = p
}

//func (g *Gol) setWorld(w [][]uint8){
//	g.Mut.Lock(); defer g.Mut.Unlock()
//	g.World = w
//}

func (g *Gol) setTurn(t int) {
	g.Mut.Lock()
	defer g.Mut.Unlock()
	g.Turn = t
}

func (g *Gol) setDone(d chan bool) {
	g.Mut.Lock()
	defer g.Mut.Unlock()
	g.Done = d
}

func (g *Gol) setReadSlice(s [][]uint8) {
	g.Mut.Lock()
	defer g.Mut.Unlock()
	g.Slice.Read = s
}

func (g *Gol) setID(id int) {
	g.Mut.Lock()
	defer g.Mut.Unlock()
	g.ID = id
}

func (g *Gol) connectWorkers(above, below string) (err error) {
	fmt.Println("Dialing above", above, "and below", below)
	g.WorkerAbove, err = rpc.Dial("tcp", above)
	g.WorkerBelow, err = rpc.Dial("tcp", below)
	return
}

func (g *Gol) Setup(req stubs.SetupRequest, res *stubs.SetupResponse) (err error) {
	runningCalls.Add(1)
	defer runningCalls.Done()

	fmt.Println("Setting up")

	resetGol(g)
	g.setID(req.ID)
	g.IsIDEven = (req.ID + 2) % 2 == 0
	//g.WaitForReadCh = make(chan bool, 1) //always buffer with 1 so rpc to add to it doesnt block
	// fmt.Println(g.ID, req.ID)

	sliceSize := len(req.Slice)
	g.TopHalo = req.Slice[0]
	g.BottomHalo = req.Slice[sliceSize-1] //by reference, so we need the mutex
	g.TopHalosCh = make(chan []uint8, 0)
	g.BottomHalosCh = make(chan []uint8, 0)

	g.Slice = NewSwapSlice(g, req.Slice)
	active = Active{
		Top:         1,             //getting rid of a row
		Bottom:      sliceSize - 2, //going to zero index, then getting rid of a row
		AliveOffset: req.Offset,
	} //top and bottom index of the part we write to

	fmt.Println("Setup SLICE IS", len(g.Slice.Read), "LONG")
	//showMatrix(g.Slice.Read)

	g.ReadOnlySlice = func(x, y int) uint8 {
		//fmt.Println("Closure: SLICE IS", len(g.Slice.Read), "LONG")
		//fmt.Println("Closure for (",x,",",y,") on ROW", y, "IS", len(g.Slice.Read[y]), "LONG")
		return g.Slice.Read[y][x]
	}

	g.setParams(req.Params)

	err = g.connectWorkers(req.Above, req.Below)
	g.IsAboveEven = req.IsAboveEven
	g.IsBelowEven = req.IsBelowEven
	// fmt.Println(g.ID, g.Params, g.Slice)

	res.ID = req.ID
	res.Success = err == nil
	return
}

//func (g *Gol) lockOnParity() {
//	if (g.ID+2)%2 == 0 {
//		fmt.Println("Block on channel on turn", g.Turn)
//		<-g.WaitForReadCh //taketurns will block on this lock until it has been read from, if it is even
//	}
//	return
//}

func (g *Gol) GetHaloRow(req stubs.HaloRequest, res *stubs.HaloResponse) (err error) {
	fmt.Println("ID", req.CallerID, "asks GetHaloRow(); Top:", req.Top)
	if req.Top {
		res.Halo = <- g.TopHalosCh //responds with the top of its writing-to slice (not /its/ halo rows)
	} else {
		res.Halo = <- g.BottomHalosCh //bottom
	}

	return
}

func (g *Gol) requestHalo(worker *rpc.Client) []uint8 {
	top := !(worker == g.WorkerAbove)
	reqAbove := stubs.HaloRequest{Top: top, CallerID: g.ID} //want the first processed row, not the outdated halo
	resAbove := new(stubs.HaloResponse)
	err := worker.Call(stubs.GetHaloHandler, reqAbove, resAbove)
	if err != nil {
		panic(err)
	}
	return resAbove.Halo
}

func (g *Gol) requestHalos() ([]uint8, []uint8){
	var aboveHalo []uint8
	var belowHalo []uint8
	if g.IsAboveEven && !g.IsBelowEven { //we're at the top of the image and g.ID=0
		//send then receive
		fmt.Println("sending")
		sendHaloAndBlock(g.Slice.Read[active.Top], g.TopHalosCh)
		fmt.Println("requesting")
		aboveHalo = g.requestHalo(g.WorkerAbove)

		belowHalo = g.requestHalo(g.WorkerBelow) //remaining call to odd worker
	}else if g.IsBelowEven && !g.IsAboveEven{
		//receive then send
		fmt.Println("requesting")
		belowHalo = g.requestHalo(g.WorkerBelow)
		fmt.Println("sending")
		sendHaloAndBlock(g.Slice.Read[active.Bottom], g.BottomHalosCh)

		aboveHalo = g.requestHalo(g.WorkerAbove) //remaining call to odd worker
	} else /* if g.IsAboveEven == g.IsBelowEven */ {
		fmt.Println("requesting x2")
		aboveHalo = g.requestHalo(g.WorkerAbove)
		belowHalo = g.requestHalo(g.WorkerBelow)
	}
	return aboveHalo, belowHalo
}

func sendHaloAndGo(h []uint8, ch chan []uint8){
	go func(){ ch <- h }()
}

func sendHaloAndBlock(h []uint8, ch chan []uint8){
	ch <- h
}

func (g *Gol) presentHalos() { //put rows on channels that block when facing odd workers
	if g.IsAboveEven && !g.IsBelowEven { //at the top of the image
		// then we cant block and wait for it to read on the channel facing the even worker
		sendHaloAndBlock(g.Slice.Read[active.Bottom], g.BottomHalosCh) // will block until odd has read it
		//sendHaloAndBlock(g.Slice.Read[active.Top], g.TopHalosCh)
	}else if g.IsBelowEven && !g.IsAboveEven {
		sendHaloAndBlock(g.Slice.Read[active.Top], g.TopHalosCh)
		//sendHaloAndBlock(g.Slice.Read[active.Bottom], g.BottomHalosCh)
	} else {
		sendHaloAndBlock(g.Slice.Read[active.Top], g.TopHalosCh)
		sendHaloAndBlock(g.Slice.Read[active.Bottom], g.BottomHalosCh)
	}
}

//func (g *Gol) UnlockIfEven(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error){
//	if (g.ID + 2) % 2 == 0 {
//		g.WaitForReadCh <- true
//		fmt.Println("Unlocked through RPC because I am even, and locked myself!")
//	}
//
//	return
//}

func showMatrix(m [][]uint8) {
	for _, row := range m {
		for _, elem := range row {
			var str string
			if elem == 255 {
				str = "##"
			} else {
				str = "[]"
			}
			fmt.Printf("%s", str)
		}
		fmt.Println("")
	}
	return
}

func writeIntoSlice(src, dst []uint8) {
	for i, _ := range src {
		dst[i] = src[i]
	}
	return
}

//RPC methods
func (g *Gol) TakeTurns(req stubs.Request, res *stubs.Response) (err error) {
	runningCalls.Add(1)
	defer runningCalls.Done()

	//fmt.Println(g)

	for i := 0; i < req.Params.Turns; i++ {
		//g.setWorld(req.World)
		fmt.Println("____________ Turn", g.Turn, "____________")
		g.SliceMut.Lock()
		calculateNextStateHalo(g, g.Params, g.ReadOnlySlice)
		g.Slice.setReadToWrite(g) //sets newly written g.Slice.Write to g.Slice.Read
		//showMatrix(g.Slice.Read)
		//showMatrix(g.Slice.Write)
		// all following methods that depend on g.Slice must read from g.Slice.Read
		g.SliceMut.Unlock()

		//g.advertiseTurnComplete() //blocks if nobody is trying to read our rows, blocks until somebody says they want to

		g.TurnMut.Lock()
		g.setTurn(g.Turn + 1)
		g.TurnMut.Unlock()

		if g.IsIDEven { //if its not we present after we ask
			fmt.Println("Waiting to be read from by my odd peers")
			g.presentHalos()
		}
		fmt.Println("Proceeding...")

		//then we read from others
		above, below := g.requestHalos()

		if !g.IsIDEven { //if its not we present after we ask
			fmt.Println("Waiting to be read from by my even peers")
			g.presentHalos()
		}
		//<-g.WaitForReadCh    //makes sure the node that depends on me can read from me before i take off again
		//if (g.ID+2)%2 == 1 { //symmetrical extra receive from the other node that depend on it if odd
		//	fmt.Println("Waiting Finally...")
		//	<-g.WaitForReadCh
		//}
		fmt.Println("Received both external reads, updating rows...")

		//write the new data into our slice
		g.SliceMut.Lock()

		writeIntoSlice(above, g.TopHalo)
		writeIntoSlice(below, g.BottomHalo)

		g.SliceMut.Unlock()

		//fmt.Println(g.TopHalo)
		//fmt.Println(g.BottomHalo)
	}

	g.Mut.Lock()

	//showMatrix(g.Slice.Read)

	res.Slice = g.Slice.Read[active.Top : active.Bottom+1] //remove non-active stale ghost/halo rows (need to add one as exclusive)
	fmt.Println("SLICEOUT IS", len(res.Slice), "LONG")
	//showMatrix(res.Slice)
	res.Turn = g.Turn
	res.Alive = g.aliveStrip(g.ReadOnlySlice)
	res.ID = g.ID

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

func (g *Gol) ReportAlive(req stubs.EmptyRequest, res *stubs.AliveResponse) (err error) {
	runningCalls.Add(1)
	defer runningCalls.Done()

	g.WorldMut.Lock()
	defer g.WorldMut.Unlock()
	g.TurnMut.Lock()
	defer g.TurnMut.Unlock()
	res.Alive = g.aliveStrip(g.ReadOnlySlice)

	res.OnTurn = g.Turn

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
func (g *Gol) Finish(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error) {
	runningCalls.Add(1)
	defer runningCalls.Done()

	g.Mut.Lock()
	g.Done <- true
	g.Mut.Unlock()

	return
}

// lets the server know that it needs to shut down as soon as possible
// returns the number of currently running rpc calls by reading the value of the waitgroup (will always return at least 1, since it includes itself)
func (g *Gol) Kill(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error) {
	runningCalls.Add(1)
	defer runningCalls.Done()

	kill <- true
	fmt.Println("server set to close when ready")
	return
}

func runServer(s *rpc.Server, l *net.Listener) {
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
	if err != nil {
		panic(err)
	}
	fmt.Println("server listening on port " + *portPtr)

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
