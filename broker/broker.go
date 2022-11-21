package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
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
	WorldA [][]byte // this is an optimisation that reduces the number of memory allocations on each turn
	WorldB [][]byte // these worlds take turns to be the next world being written into
	IsCurrentA bool
	CurrentWorldPtr *[][]byte
	NextWorldPtr *[][]byte
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

func (b *Broker) alternateWorld() {
	if b.IsCurrentA {
		b.CurrentWorldPtr = &b.WorldB
		b.NextWorldPtr = &b.WorldA
	}else {
		b.CurrentWorldPtr = &b.WorldA
		b.NextWorldPtr = &b.WorldB
	}
	b.IsCurrentA = !b.IsCurrentA
}

func (b *Broker) getCurrentWorld() [][]byte{
	return *b.CurrentWorldPtr
}

func (b *Broker) getNextWorld() [][]byte{
	return *b.CurrentWorldPtr
}

func (b *Broker) getAliveCells() {
	for _, worker := range b.Workers {
		aliveRes := new(stubs.AliveResponse)
		worker.Connection.Call(stubs.AliveHandler, stubs.EmptyRequest{}, aliveRes)
		b.Alive = append(b.Alive, aliveRes.Alive...)
	}
}

func (b *Broker) AcceptClient (req stubs.NewClientRequest, res *stubs.NewClientResponse) (err error) {
	//threads
	//world
	//turns

	b.WorldsMut.Lock()
	b.CurrentWorldPtr = &b.WorldA
	b.NextWorldPtr = &b.WorldB
	*b.CurrentWorldPtr = req.World ///deref currentworld in order to change its actual content to the new world
	*b.NextWorldPtr = req.World // to be overwritten
	b.WorldsMut.Unlock()

	b.Params = req.Params
	b.Threads = req.Params.Threads

	b.TurnsMut.Lock()
	b.Turns = req.Params.Turns
	b.TurnsMut.Unlock()

	//send work to the gol workers
	workSpread := spreadWorkload(b.Params.ImageHeight, b.Threads)
	workers := takeWorkers(b)

	if len(workers) == 0 { return } //let client know that there are no workers available

	for workerId := 0; workerId < len(workers); workerId++ {
		//connect to the worker
		client, err := rpc.Dial("tcp", workers[workerId].Ip)
		handleError(err)
		workers[workerId].Connection = client

	}

	for workerId := 0; workerId < len(workers); workerId++ {
		worker := workers[workerId]
		y1 := workSpread[workerId]; y2 := workSpread[workerId+1]

		setupReq := stubs.SetupRequest{ID: workerId, Slice: stubs.Slice{From: y1, To: y2}, Params: b.Params, World: req.World}
		err = worker.Connection.Call(stubs.SetupHandler, setupReq, new(stubs.SetupResponse))

		handleError(err)
	}

	noWorkers := len(b.Workers)



	out := make(chan *stubs.Response)

	b.getAliveCells()
	fmt.Println(b.Turns)
	i := 0
	for i < b.Turns {
		turnResponses := make([]stubs.Response, noWorkers)

		//send a turn request to each worker selected
		for _, worker := range workers {
			turnReq := stubs.Request{World: b.getCurrentWorld()}

			//receive response when ready (in any order) via the out channel
			go func(){
				turnRes := new(stubs.Response)
				worker.Connection.Call(stubs.TurnHandler, turnReq, turnRes)
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
			strip := response.Strip
			for _, row := range strip {
				(*b.NextWorldPtr)[rowNum] = row
				rowNum++
			}

		}
		b.alternateWorld()
		res.Turns++
		//reconstruct the world to go again

		b.getAliveCells()
		b.WorldsMut.Unlock()
		b.TurnsMut.Lock()
		i++
		b.TurnsMut.Unlock()
	}
	res.World = req.World



	//close the workers after we're finished
	for _, worker := range workers {
		worker.Connection.Close()
	}

	res.Alive = b.Alive

	fmt.Println(b.Turns, len(b.Alive))

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

	workers := make([]Worker, 3)
	workers[0] = Worker{Ip: "localhost:8032"}
	//workers[1] = Worker{Ip: "localhost:8033"}
	//workers[2] = Worker{Ip: "localhost:8034"}

	rpc.Register(&Broker{Workers: workers, IsCurrentA: true})
	listener, err := net.Listen("tcp", ":"+*pAddr) //listening for the client
	fmt.Println("Listening on ", *pAddr)

	handleError(err)
	defer listener.Close()
	rpc.Accept(listener)

}