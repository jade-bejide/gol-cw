package gol

import (
	"flag"
	"net"
	"net/rpc"
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
type Broker struct {
	Threads int
	World [][]byte
	Turns int
	Workers []string //have 16 workers by default, as this is the max size given in tests
}

func handleError(err error) {
	if err != nil {
		panic(err)
	}
}

func distributeWork() {

}

func (b *Broker) AcceptClient (req stubs.NewClientRequest, res *stubs.NewClientResponse) (err error) {
	//threads
	//world
	//turns
	b.Threads = req.Threads
	b.Turns = req.Turns
	b.World = req.World

	//send work to the gol workers

	//res = b.World

	return
}
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	workers := make([]string, 3)
	workers[0] = "ip1"
	workers[1] = "ip2"
	workers[2] = "ip3"

	rpc.Register(&Broker{Workers: workers})
	listener, err := net.Listem("tcp", ":"+*pAddr)

	handleError(err)
	defer listener.Close()
	rpc.Accept(listener)

}