package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"strconv"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
	"strings"
)

var kill = make(chan bool, 1)
var finishTurns = make(chan bool, 1) //to safely quit the Gol for loop
var runningCalls = sync.WaitGroup{}
var workerIPs []string

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
	CurrentWorldPtr *[][]byte
	Turns int
	Workers []Worker //have 16 workers by default, as this is the max size given in tests
	Params stubs.Params
	Alive []util.Cell
	AliveMut sync.Mutex
	AliveTurn int
	AliveTurnMut sync.Mutex
	OnTurn int
	Idle bool
}

func (b *Broker) brokerDebug() {
	fmt.Println("BROKER DEBUG")

	fmt.Println("Threads", b.Threads)
	fmt.Println("Turns", b.Turns)
	fmt.Println("Current Turn", b.OnTurn)
	fmt.Println("Idle", b.Idle)
	fmt.Println("Params", b.Params)

	fmt.Println()
}

func handleError(err error) {
	if err != nil {
		panic(err)
	}
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

func (b *Broker) getCurrentWorld() [][]byte{
	b.WorldsMut.Lock(); defer b.WorldsMut.Unlock()
	return *b.CurrentWorldPtr
}


func (b *Broker) getAliveCells(workers []Worker) ([]util.Cell, int) { //mutex locks aren't helpful here when seting global variabls of broker
	//fmt.Println(b.Workers)
	b.TurnsMut.Lock(); defer b.TurnsMut.Unlock() //sync with pause
	alive := make([]util.Cell, 0)
	var onTurn int
	for workerId := 0; workerId < b.Threads; workerId++  {
		workers[workerId].Lock.Lock()
		aliveRes := new(stubs.AliveResponse)
		workers[workerId].Connection.Call(stubs.AliveHandler, stubs.EmptyRequest{}, aliveRes)
		workers[workerId].Lock.Unlock()
		alive = append(alive, aliveRes.Alive...)
		onTurn = aliveRes.OnTurn

		fmt.Println(onTurn)
	}

	return alive, onTurn
}

//SDL Key Presses RPCs
func (b *Broker) SaveWorld(req stubs.EmptyRequest, res *stubs.WorldResponse) (err error) {
	runningCalls.Add(1); defer runningCalls.Done()
	
	b.TurnsMut.Lock(); defer b.TurnsMut.Unlock()

	res.World = b.getCurrentWorld()
	res.OnTurn = b.OnTurn

	return
}

func (b *Broker) PauseGol(req stubs.PauseRequest, res *stubs.PauseResponse) (err error) {
	runningCalls.Add(1); defer runningCalls.Done()
	
	if req.Pause {
		b.TurnsMut.Lock(); b.WorldsMut.Lock() 
		res.Turns = b.OnTurn	
	} else { 
		res.Turns = b.OnTurn
		b.TurnsMut.Unlock(); b.WorldsMut.Unlock() 
	}

	return
}



//connect to the workers in a loop
func (b *Broker) setUpWorkers() (issue string) {
	b.Workers = make([]Worker, b.Threads)
	for i := 0; i < b.Threads; i++ {
		fmt.Println("Loop")
		b.Workers[i].Lock.Lock()
		b.Workers[i].Ip = "localhost:"+strconv.Itoa(8032+i)
		b.Workers[i].Ip = workerIPs[i];


		fmt.Println("Dials", workerIPs[i])
		client, err := rpc.Dial("tcp", b.Workers[i].Ip)

		handleError(err)
		if err != nil {
			b.Workers[i].Lock.Unlock()
			fmt.Println("Failed to dial")
			issue = err.Error()
			return
		}

		//handleError(err)
		b.Workers[i].Connection = client
		b.Workers[i].Lock.Unlock()
	}

	return
}

func (b *Broker) getCurrentAliveCells() []util.Cell {
	b.AliveMut.Lock(); defer b.AliveMut.Unlock()

	return b.Alive
}

func (b *Broker) getTurn() int {
	b.TurnsMut.Lock(); defer b.TurnsMut.Unlock()

	return b.OnTurn
}

func (b *Broker) KillBroker(req stubs.EmptyRequest, res *stubs.KillBrokerResponse) (err error) {
	runningCalls.Add(1); defer runningCalls.Done()

	
	b.WorldsMut.Lock(); b.TurnsMut.Lock();
	finishTurns <- true
	for workerId := 0; workerId < b.Threads; workerId++ {
		b.Workers[workerId].Lock.Lock()

		fmt.Println("Attempting to kill worker", workerId)
		b.Workers[workerId].Connection.Call(stubs.KillHandler, stubs.EmptyRequest{}, &stubs.EmptyResponse{})
		// handleError(err)
		b.Workers[workerId].Connection.Close()
		fmt.Println("Killed worker", workerId)

		b.Workers[workerId].Lock.Unlock()
	}

	res.Alive = b.getCurrentAliveCells()
	res.OnTurn = b.OnTurn

	b.WorldsMut.Unlock(); b.TurnsMut.Unlock()

	kill <- true
	
	fmt.Println("Set to close when ready")
	return
}

func (b *Broker) Finish(req stubs.EmptyRequest, res *stubs.QuitWorldResponse) (err error) {
	runningCalls.Add(1); defer runningCalls.Done()
	
	//finish itself
	b.TurnsMut.Lock()
	b.WorldsMut.Lock()
	b.Idle = true
	
	//call all the servers to finish
	for workerId := 0; workerId < b.Threads; workerId++ {
		b.Workers[workerId].Lock.Lock()

		err := b.Workers[workerId].Connection.Call(stubs.FinishHander, stubs.EmptyRequest{}, new(stubs.EmptyResponse))
		handleError(err)

		b.Workers[workerId].Lock.Unlock()
	}

	res.OnTurn = b.OnTurn
	res.Alive = b.Alive

	fmt.Println("Going to sleep.")


	return
}


//fault tolerance
func (b *Broker) wakeUp() {
	fmt.Println("Waking up")
	b.Idle = false
	b.setUpWorkers()
	b.TurnsMut.Unlock()
	b.WorldsMut.Unlock()
}

func (b *Broker) setCurrentWorldRow(rowIndex int, row []byte) {
	// b.WorldsMut.Lock(); defer b.WorldsMut.Unlock()
	(*b.CurrentWorldPtr)[rowIndex] = row
}

func (b *Broker) getCurrentTurn() int {
	b.TurnsMut.Lock(); defer b.TurnsMut.Unlock()

	return b.OnTurn
}

func (b *Broker) checkWorkerAddresses(threads int) (issue string) {
	//fmt.Println("what")

	if threads > len(workerIPs) {
		//fmt.Println("not enough worker addresses")
		return "not enough addresses"
	}

	//fmt.Println("workers set up")
	issue = b.setUpWorkers()
	return issue
}

func (b *Broker) AcceptClient (req stubs.NewClientRequest, res *stubs.NewClientResponse) (err error) {
	runningCalls.Add(1); defer runningCalls.Done()
	var i int


	b.brokerDebug()

	if b.Idle { 
		b.wakeUp()

		if !req.Continue {
			
			b.WorldsMut.Lock()
			b.CurrentWorldPtr = &b.WorldA
			*b.CurrentWorldPtr = req.World ///deref currentworld in order to change its actual content to the new world
			b.WorldsMut.Unlock()
		
			b.Params = req.Params
			b.Threads = req.Params.Threads

			b.TurnsMut.Lock()
			b.Turns = req.Params.Turns
			b.TurnsMut.Unlock()



			i = 0
		} else { 

			fmt.Println("Continuing")
			i = b.getCurrentTurn()
		}
	} else {
		b.WorldsMut.Lock()
		b.CurrentWorldPtr = &b.WorldA
		*b.CurrentWorldPtr = req.World ///deref currentworld in order to change its actual content to the new world
		b.WorldsMut.Unlock()
	
		b.Params = req.Params
		b.Threads = req.Params.Threads
	
		b.TurnsMut.Lock()
		b.Turns = req.Params.Turns
		b.TurnsMut.Unlock()



		i = 0
	}


	issue := b.checkWorkerAddresses(req.Params.Threads)
	if(issue != ""){
		fmt.Println("Error checking workers & addresses:", issue)
		res.Alive = []util.Cell{}
		res.Turns = -1
		res.World = [][]uint8{}
		return
	}


	workers := b.Workers

	// workers := takeWorkers(b)
	

	// if len(workers) == 0 { return } //let client know that there are no workers available

	noWorkers := len(b.Workers)

	out := make(chan *stubs.Response, b.Threads)

	if b.Params.Turns == 0 {
		res.Alive, _ = b.getAliveCells(workers)
		res.Turns = b.Turns
		res.World = b.getCurrentWorld()
		return
	}


	//send work to the gol workers
	workSpread := spreadWorkload(b.Params.ImageHeight, b.Threads)

	for workerId := 0; workerId < len(workers); workerId++ {
		y1 := workSpread[workerId]; y2 := workSpread[workerId+1]

		setupReq := stubs.SetupRequest{ID: workerId, Slice: stubs.Slice{From: y1, To: y2}, Params: b.Params, World: b.getCurrentWorld(), Turn: i}
		workers[workerId].Lock.Lock()
		err = workers[workerId].Connection.Call(stubs.SetupHandler, setupReq, new(stubs.SetupResponse))
		workers[workerId].Lock.Unlock()

		handleError(err)
	}



	exitLoop := false
	for i < b.Turns && !exitLoop {
		select {
			case <-finishTurns:
				exitLoop = true
			default:
				turnResponses := make([]stubs.Response, noWorkers)
				//send a turn request to each worker selected
				for workerId := 0; workerId < b.Threads; workerId++ {
					turnReq := stubs.Request{World: b.getCurrentWorld()}
					//receive response when ready (in any order) via the out channel
					go func(workerId int){
						turnRes := new(stubs.Response)
						// done := make(chan *rpc.Call, 1)
						workers[workerId].Lock.Lock()
						workers[workerId].Connection.Call(stubs.TurnHandler, turnReq, turnRes)
						workers[workerId].Lock.Unlock()
						out <- turnRes
					}(workerId)
				}



				//gather the work piecewise
				for worker := 0; worker < b.Threads; worker++ {
					turnRes := <-out
					turnResponses[turnRes.ID] = *turnRes
				}


				rowNum := 0
				
				b.WorldsMut.Lock()

				for responseId := 0; responseId < len(turnResponses); responseId++ {
					strip := turnResponses[responseId].Strip 
					for rowId := 0; rowId < len(strip); rowId++ {
						b.setCurrentWorldRow(rowNum, strip[rowId])
						rowNum++
					}
				}

				b.WorldsMut.Unlock()

				res.Turns++
				//reconstruct the world to go again

				b.AliveMut.Lock()
				b.AliveTurnMut.Lock()
				b.Alive, b.AliveTurn = b.getAliveCells(workers)
				b.AliveMut.Unlock()
				b.AliveTurnMut.Unlock()

				// b.WorldsMut.Unlock()
				b.TurnsMut.Lock()
				i++
				b.OnTurn = i
				b.TurnsMut.Unlock()
		}
	}

	res.World = b.getCurrentWorld()

	b.AliveMut.Lock()
	res.Alive, _ = b.getAliveCells(workers)
	b.AliveMut.Unlock()

	//close the workers after we're finished
	for _, worker := range workers {
		worker.Connection.Close()
	}

	fmt.Println("Finished")

	return
}

func (b *Broker) ReportAlive(req stubs.EmptyRequest, res *stubs.AliveResponse) (err error){
	runningCalls.Add(1); defer runningCalls.Done()
	b.AliveMut.Lock(); defer b.AliveMut.Unlock()
	b.AliveTurnMut.Lock(); defer b.AliveTurnMut.Unlock()
	// b.TurnsMut.Lock(); defer b.TurnsMut.Unlock()
	res.Alive = b.Alive
	res.OnTurn = b.AliveTurn
	return
}





func main() {

	pPort := flag.String("port", "8031", "Port to listen on")
	pWorkerIPs := flag.String("worker_ips", "localhost", "Worker addresses for broker to connect to, enter as a comma separated list")

	flag.Parse()

	workerIPs = strings.Split(*pWorkerIPs, ",")
	flag.Parse()
	

	rpc.Register(&Broker{})
	listener, err := net.Listen("tcp", ":"+*pPort) //listening for the client
	fmt.Println("Listening on ", *pPort)
	
	handleError(err)
	
	go rpc.Accept(listener)
	fmt.Println("Setting up workers")
	// broker.setUpWorkers()
	fmt.Println("Set up workers")

	fmt.Println("Dying...")
	<-kill
	//wait for the calls to terminate before I kill myself
	runningCalls.Wait()
	fmt.Println("Dead")
	err = listener.Close()
	// handleError(err)

	fmt.Println("Close broker")
}