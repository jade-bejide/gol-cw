package gol

import (
	"flag"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
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
	WorldA [][]byte // this is an optimisation that reduces the number of memory allocations on each turn
	WorldB [][]byte // these worlds take turns to be the next world being written into
	IsCurrentA bool
	CurrentWorldPtr *[][]byte
	NextWorldPtr *[][]byte
	Turns int
	Workers []Worker //have 16 workers by default, as this is the max size given in tests
	Params stubs.Params
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

func (b *Broker) AcceptClient (req stubs.NewClientRequest, res *stubs.NewClientResponse) (err error) {
	//threads
	//world
	//turns

	b.CurrentWorldPtr = &b.WorldA
	b.NextWorldPtr = &b.WorldB
	*b.CurrentWorldPtr = req.World ///deref currentworld in order to change its actual content to the new world
	*b.NextWorldPtr = req.World // to be overwritten
	b.Params = req.Params

	//send work to the gol workers
	workSpread := spreadWorkload(b.Params.ImageHeight, b.Threads)
	workers := takeWorkers(b)

	if len(workers) == 0 { return } //let client know that there are no workers available

	for _, worker := range workers {
		//connect to the worker
		client, err := rpc.Dial("tcp", worker.Ip)
		handleError(err)
		worker.Connection = client

	}

	for workerId := 0; workerId < len(workSpread); workerId++ {
		worker := workers[workerId]
		y1 := workSpread[workerId]; y2 := workSpread[workerId+1]

		setupReq := stubs.SetupRequest{ID: workerId, Slice: stubs.Slice{From: y1, To: y2}, Params: b.Params}
		worker.Connection.Call(stubs.SetupHandler, setupReq, new(stubs.SetupResponse))
	}

	noWorkers := len(b.Workers)



	out := make(chan *stubs.Response)

	for i := 0; i < b.Turns; i++ {
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

		rowNum := 0
		for _, response := range turnResponses{
			strip := response.Strip
			for _, row := range strip {
				(*b.NextWorldPtr)[rowNum] = row
				rowNum++
			}
		}

		b.alternateWorld()
		//reconstruct the world to go again
	}
	//res.World = b.World

	//close the workers after we're finished
	for _, worker := range workers {
		worker.Connection.Close()
	}

	return
}
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	workers := make([]Worker, 3)
	workers[0] = Worker{Ip: "ip1"}
	workers[1] = Worker{Ip: "ip2"}
	workers[2] = Worker{Ip: "ip3"}

	rpc.Register(&Broker{Workers: workers, IsCurrentA: true})
	listener, err := net.Listen("tcp", ":"+*pAddr) //listening for the client

	handleError(err)
	defer listener.Close()
	rpc.Accept(listener)

}