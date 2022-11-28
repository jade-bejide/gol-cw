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
	g.Slice.Mut.Lock(); defer g.Slice.Mut.Unlock()

	for x := 0; x < p.ImageWidth; x++ {
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

func (g *Gol) aliveStrip(worldReadOnly func(x, y int) uint8) []util.Cell {
	var cells []util.Cell

	g.Slice.Mut.Lock(); defer g.Slice.Mut.Unlock()

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

	g.setParams(stubs.Params{})
	//g.setWorld(make([][]uint8, 0))
	g.setTurn(0)
	g.setDone(make(chan bool, 1))
}

//set each element in dst to that of src, must be equal size, non-zero length and rectangular
func copyEqualSizeSlice(src, dst [][]uint8) {
	h := len(src)
	for y := 0; y < h; y++ {
		copy(dst[y], src[y])
	}
}

type SwapSlice struct {
	Mut sync.Mutex
	Read  [][]uint8
	Write [][]uint8 //Write pertains only to writing during GOL logic, other times Read is written to and read from
}

func NewSwapSlice(g *Gol, s [][]uint8) *SwapSlice {
	write := make([][]uint8, len(s))
	for i, _ := range write {
		write[i] = make([]uint8, len(s[0]))
	}

	ss := SwapSlice{
		Read:  s,
		Write: write, //wraps new world in halos
	}
	return &ss
}
func (s *SwapSlice) setReadToWrite(g *Gol) {
	s.Mut.Lock(); defer s.Mut.Unlock()
	copyEqualSizeSlice(s.Write, s.Read)
}

type Gol struct {
	Mut      sync.Mutex
	WorldMut sync.Mutex
	TurnMut  sync.Mutex

	Params   stubs.Params
	ID       int
	IsIDEven bool

	//Slice [][]uint8 //active part of the slice is all but the first and last row
	Slice         *SwapSlice
	ReadOnlySlice func(x, y int) uint8

	//for turn advertisation, they must be 0 buffered else it will allow the worker to take turns ahead of their co-workers
	TopHalosCh    chan []uint8
	BottomHalosCh chan []uint8
	RequestedHalos sync.WaitGroup

	Acknowledge chan bool

	WorkerAbove *rpc.Client
	WorkerBelow *rpc.Client
	IsAboveEven bool
	IsBelowEven bool

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

	sliceSize := len(req.Slice)

	g.TopHalosCh = make(chan []uint8, 0)
	g.BottomHalosCh = make(chan []uint8, 0)
	g.Acknowledge = make(chan bool, 0)

	g.Slice = NewSwapSlice(g, req.Slice)
	active = Active{
		Top:         1,             //getting rid of a row
		Bottom:      sliceSize - 2, //going to zero index, then getting rid of a row
		AliveOffset: req.Offset,
	} //top and bottom index of the part we write to

	g.ReadOnlySlice = func(x, y int) uint8 {
		return g.Slice.Read[y][x]
	}

	g.setParams(req.Params)

	err = g.connectWorkers(req.Above, req.Below)
	g.IsAboveEven = req.IsAboveEven
	g.IsBelowEven = req.IsBelowEven

	res.ID = req.ID
	res.Success = err == nil
	return
}

func (g *Gol) GetHaloRow(req stubs.HaloRequest, res *stubs.HaloResponse) (err error) {

	defer g.RequestedHalos.Done()
	if req.Top {
		res.Halo = <- g.TopHalosCh //responds with the top of its writing-to slice (not /its/ halo rows)
	} else {
		res.Halo = <- g.BottomHalosCh  //bottom
	}
	return
}

func (g *Gol) Ack(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error) {
	g.Acknowledge <- true
	return
}

func (g *Gol) requestHalo(worker *rpc.Client) []uint8 {
	top := worker == g.WorkerBelow
	reqAbove := stubs.HaloRequest{Top: top, CallerID: g.ID} //want the first processed row, not the outdated halo
	resAbove := new(stubs.HaloResponse)
	err := worker.Call(stubs.GetHaloHandler, reqAbove, resAbove)
	if err != nil {
		panic(err)
	}
	return resAbove.Halo
}

func (g *Gol) requestHalos() ([]uint8, []uint8){
	g.Slice.Mut.Lock(); defer g.Slice.Mut.Unlock()
	var aboveHalo []uint8
	var belowHalo []uint8
	if g.IsAboveEven && !g.IsBelowEven { //we're at the top of the image and g.ID=0
		sendHaloAndBlock(g.Slice.Read[active.Top], g.TopHalosCh)
		aboveHalo = g.requestHalo(g.WorkerAbove)

		belowHalo = g.requestHalo(g.WorkerBelow) //remaining call to odd worker
	}else if g.IsBelowEven && !g.IsAboveEven{
		belowHalo = g.requestHalo(g.WorkerBelow)
		sendHaloAndBlock(g.Slice.Read[active.Bottom], g.BottomHalosCh)

		aboveHalo = g.requestHalo(g.WorkerAbove) //remaining call to odd worker
	} else /* if g.IsAboveEven == g.IsBelowEven */ {
		belowHalo = g.requestHalo(g.WorkerBelow)
		aboveHalo = g.requestHalo(g.WorkerAbove) //above must be requested after, so as to mirror the order of advertisation
	}
	return aboveHalo, belowHalo
}

func sendHaloAndBlock(h []uint8, ch chan []uint8){
	ch <- h
}

func (g *Gol) presentHalos() { //put rows on channels that block when facing odd workers
	g.Slice.Mut.Lock(); defer g.Slice.Mut.Unlock()
	if g.IsAboveEven && !g.IsBelowEven { //at the top of the image
		// then we cant block and wait for it to read on the channel facing the even worker
		sendHaloAndBlock(g.Slice.Read[active.Bottom], g.BottomHalosCh) // will block until odd has read it
	}else if g.IsBelowEven && !g.IsAboveEven {
		sendHaloAndBlock(g.Slice.Read[active.Top], g.TopHalosCh)
	} else {
		sendHaloAndBlock(g.Slice.Read[active.Top], g.TopHalosCh)
		sendHaloAndBlock(g.Slice.Read[active.Bottom], g.BottomHalosCh)
	}
}

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

func (g *Gol) synchronise() {
	//fmt.Println("Lets synchronise!")
	if g.IsIDEven {
		if !g.IsAboveEven {
			//fmt.Println("waiting 1")
			<-g.Acknowledge
			//fmt.Println("waiting 1.5")
			go g.WorkerAbove.Call(stubs.AckHandler, stubs.EmptyRequest{}, new(stubs.EmptyResponse))
		} //wait for odd to request from us
		if !g.IsBelowEven {
			//fmt.Println("waiting 2")
			<-g.Acknowledge
			//fmt.Println("waiting 2.5")
			go g.WorkerBelow.Call(stubs.AckHandler, stubs.EmptyRequest{}, new(stubs.EmptyResponse))
		} //wait for odd to request from us
	}else{ //always expect two back as odd
		//fmt.Println("waiting 3")
		go g.WorkerAbove.Call(stubs.AckHandler, stubs.EmptyRequest{}, new(stubs.EmptyResponse))
		//fmt.Println("waiting 3")
		<-g.Acknowledge

		//fmt.Println("waiting 4")
		go g.WorkerBelow.Call(stubs.AckHandler, stubs.EmptyRequest{}, new(stubs.EmptyResponse))
		//fmt.Println("waiting 4")
		<-g.Acknowledge
	}
	g.TurnMut.Lock()
	fmt.Printf("Synchronised with neighbours on turn %d\n", g.Turn)
	g.TurnMut.Unlock()
}

//RPC methods
func (g *Gol) TakeTurns(req stubs.Request, res *stubs.Response) (err error) {
	runningCalls.Add(1)
	defer runningCalls.Done()

	for i := 0; i < req.Params.Turns; i++ {
		calculateNextStateHalo(g, g.Params, g.ReadOnlySlice)
		g.Slice.setReadToWrite(g) //sets newly written g.Slice.Write to g.Slice.Read

		g.TurnMut.Lock()
		g.setTurn(g.Turn + 1)
		g.TurnMut.Unlock()

		g.RequestedHalos.Add(2) //we require EXACTLY 2 reads externally to fully complete before we progress

		// let ourselves be read from FIRST if even (may present only one halo with odd workers)
		if g.IsIDEven {
			g.presentHalos()
		}

		// we read from others (straight away if odd)
		above, below := g.requestHalos()

		// present all our halos as odd
		if !g.IsIDEven { //if its not we present after we ask
			g.presentHalos()
		}

		g.RequestedHalos.Wait() //if we do not wait here, we may end up writing to the data we return from GetHaloRow
		g.synchronise()

		//write the new data into our slice
		g.Slice.Mut.Lock()
		writeIntoSlice(above, g.Slice.Read[0])
		writeIntoSlice(below, g.Slice.Read[len(g.Slice.Read) - 1])
		g.Slice.Mut.Unlock()

	}

	g.Mut.Lock()

	res.Slice = g.Slice.Read[active.Top : active.Bottom+1] //remove non-active stale ghost/halo rows (need to add one as exclusive)
	res.Turn = g.Turn
	res.Alive = g.aliveStrip(g.ReadOnlySlice)
	res.ID = g.ID

	g.Mut.Unlock()

	return
}

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
