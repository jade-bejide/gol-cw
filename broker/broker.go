package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

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

type ClientTask struct {
	Client *rpc.Client
	Threads int
	World [][]byte
	Turns int
}

type Worker struct {
	Ip string
	Working bool
	Lock sync.Mutex
	Connection *rpc.Client
	Done chan *rpc.Call
}

type Broker struct {
	Threads int
	WorldsMut sync.Mutex
	TurnsMut sync.Mutex
	InWorld [][]uint8
	OutWorld [][]uint8
	// no longer needs to write the world to our struct
	//WorldA [][]byte // this is an optimisation that reduces the number of memory allocations on each turn
	//WorldB [][]byte // these worlds take turns to be the next world being written into
	//IsCurrentA bool
	//CurrentWorldPtr *[][]byte
	//NextWorldPtr *[][]byte
	Turns int
	Workers []Worker //have 16 workers by default, as this is the max size given in tests
	Params stubs.Params
	Alive []util.Cell
}

func handleError(err error) {
	if err != nil {
		panic(err)
	}
}

func distributeWork() {

}

func takeWorkers(b *Broker) []Worker {
	threads := b.Threads
	workers := make([]Worker, 0)

	totalworkers := 0
	for _, worker := range b.Workers {
		worker.Lock.Lock()
		if !worker.Working {
			workers = append(workers, worker)
			worker.Working = true
			totalworkers++
		}
		worker.Lock.Unlock()

		if totalworkers == threads { return workers }
	}

	return make([]Worker, 0) //if not all workers are available, no workers are available
}

func (b *Broker) getAliveCells(workers []Worker) {
	//fmt.Println(b.Workers)
	b.Alive = make([]util.Cell, 0)
	for _, worker := range workers {
		aliveRes := new(stubs.AliveResponse)
		worker.Connection.Call(stubs.AliveHandler, stubs.EmptyRequest{}, aliveRes)
		b.Alive = append(b.Alive, aliveRes.Alive...)
	}
}

func (b *Broker) setUpWorkers() {
	b.Workers = make([]Worker, b.Threads)
	for i := 0; i < b.Threads; i++ {
		b.Workers[i].Ip = "localhost:"+strconv.Itoa(8032+i)
	}
}

func (b *Broker) getHalos(y1 int, y2 int) ([]byte, []byte) {
	size := b.Params.ImageHeight
	var topHalo []byte
	var bottomHalo []byte

	b.WorldsMut.Lock()
	if y1 == 0 && y2 == size{
		topHalo = b.InWorld[size-1]
		bottomHalo = b.InWorld[0]
	} else if y1 == 0 {
		topHalo = b.InWorld[size-1]
	} else if y2 == size {
		bottomHalo = b.InWorld[0]
	} else {
		topHalo = b.InWorld[y1-1]
		bottomHalo = b.InWorld[y2] //y2 is exclusive
	}
	b.WorldsMut.Unlock()

	return topHalo, bottomHalo
}

func (b *Broker) AcceptClient (req stubs.NewClientRequest, res *stubs.NewClientResponse) (err error) {
	//threads
	//world
	//turns

	b.WorldsMut.Lock()
	b.InWorld = req.World ///deref currentworld in order to change its actual content to the new world
	b.WorldsMut.Unlock()

	b.Params = req.Params
	b.Threads = req.Params.Threads

	b.setUpWorkers()
	fmt.Println(b.Workers)

	b.TurnsMut.Lock()
	b.Turns = req.Params.Turns
	b.TurnsMut.Unlock()

	//send work to the gol workers
	workSpread := spreadWorkload(b.Params.ImageHeight, b.Threads)
	workers := takeWorkers(b)
	// workers := b.Workers

	if len(workers) == 0 { return } //let client know that there are no workers available

	for workerId := 0; workerId < len(workers); workerId++ {
		//connect to the worker
		client, err := rpc.Dial("tcp", workers[workerId].Ip)

		handleError(err)
		workers[workerId].Connection = client

	}


	for workerId := 0; workerId < len(workers); workerId++ {
		nextId := (workerId - 1 + len(workers)) % len(workers)
		lastId := (workerId + 1 + len(workers)) % len(workers)
		above := workers[nextId].Ip
		below := workers[lastId].Ip
		worker := &workers[workerId]
		y1 := workSpread[workerId]; y2 := workSpread[workerId+1]

		setupReq := stubs.SetupRequest{
			ID: workerId,
			Slice: b.InWorld[y1:y2], //this needs to include the ghost rows
			Params: b.Params,
			Above: above,
			//in-between: this slice
			Below: below,
		}
		setupRes := new(stubs.SetupResponse)
		err = worker.Connection.Call(stubs.SetupHandler, setupReq, setupRes)
		handleError(err)
		if !setupRes.Success { //fault toll
			fmt.Println("Error workers could not find each other!")
			os.Exit(1)
		}
	}

	noWorkers := len(b.Workers)

	out := make(chan *stubs.Response, b.Threads)

	if b.Params.Turns == 0 {

		b.getAliveCells(workers)
		res.Alive = b.Alive
		res.Turns = b.Turns
		res.World = req.World
		return
	}
	fmt.Println(workers)
	i := 0

	turnResponses := make([]stubs.Response, noWorkers)
	//send a turn request to each worker selected
	for workerId := 0; workerId < len(workers); workerId++ {
		worker := &workers[workerId]
		turnReq := stubs.Request{Params: req.Params, World:}
		//receive response when ready (in any order) via the out channel
		go func(){
			turnRes := new(stubs.Response)
			// done := make(chan *rpc.Call, 1)
			err := worker.Connection.Call(stubs.TurnsHandler, turnReq, turnRes)
			if err != nil {
				handleError(err)
			}
			// <-done
			out <- turnRes
		}()
	}

	//gather the work piecewise
	for worker := 0; worker < b.Threads; worker++ {
		turnRes := <-out
		turnResponses[turnRes.ID] = *turnRes
	}

	b.Alive = make([]util.Cell, 0)
	rowNum := 0
	b.WorldsMut.Lock()
	for _, response := range turnResponses {
		slice := response.Slice
		for _, row := range slice {
			b.OutWorld[rowNum] = row
			rowNum++
		}

	}
	// b.alternateWorld()
	res.Turns++
	//reconstruct the world to go again

	b.getAliveCells(workers)
	b.WorldsMut.Unlock()
	b.TurnsMut.Lock()
	i++
	b.TurnsMut.Unlock()


	res.World = b.OutWorld



	//close the workers after we're finished
	for _, worker := range workers {
		worker.Connection.Close()
	}

	res.Alive = b.Alive


	return
}

func (b *Broker) ReportAlive(req stubs.EmptyRequest, res *stubs.AliveResponse) (err error){
	b.WorldsMut.Lock()
	b.TurnsMut.Lock()
	res.Alive = b.Alive
	res.OnTurn = b.Turns
	b.TurnsMut.Lock()
	b.WorldsMut.Unlock()

	return
}



func main() {
	pAddr := flag.String("port", "8031", "Port to listen on")
	flag.Parse()



	rpc.Register(&Broker{})
	listener, err := net.Listen("tcp", ":"+*pAddr) //listening for the client
	fmt.Println("Listening on ", *pAddr)

	handleError(err)
	defer listener.Close()
	rpc.Accept(listener)

}