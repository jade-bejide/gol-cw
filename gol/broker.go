package gol

import (
	"flag"
	"net"
	"net/rpc"
	"sync"
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
}
type Broker struct {
	Threads int
	World [][]byte
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
		worker.Lock.lock()
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



func (b *Broker) AcceptClient (req stubs.NewClientRequest, res *stubs.NewClientResponse) (err error) {
	//threads
	//world
	//turns
	b.World = req.World
	b.Params = req.Params

	//send work to the gol workers
	workSpread := spreadWorkload(b.Height, b.Threads)
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

	for i := 0; i < b.Turns; i++ {
		for _, worker := range workers {
			//turnReq :=
			worker.Connection.Call(stubs.TurnHandler)
		}

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

	rpc.Register(&Broker{Workers: workers})
	listener, err := net.Listen("tcp", ":"+*pAddr) //listening for the client

	handleError(err)
	defer listener.Close()
	rpc.Accept(listener)

}