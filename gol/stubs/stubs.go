package stubs

import "uk.ac.bris.cs/gameoflife/util"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type EmptyRequest struct {}
type EmptyResponse struct{}


var SetupHandler = "Gol.Setup"
type SetupRequest struct {
	ID int
	Offset int //the row our active slice starts in the image
	Slice [][]uint8
	Params Params
	Above string //ip address and port
	Below string //ip address and port
	IsAboveEven bool
	IsBelowEven bool
}
type SetupResponse struct {
	ID int
	Success bool
}

var HaloSetupHandler = "Gol.HaloSetup"
type HaloSetupRequest struct {
	ID int
	Params Params
	//TopHalo []byte //these can be requested before each iteration
	//BottomHalo []byte
}
//empty response

var GetHaloHandler = "Gol.GetHaloRow"
type HaloRequest struct {
	CallerID int
	Top bool
}
type HaloResponse struct {
	Halo []uint8
}

var AckHandler = "Gol.Ack"
// empty request
// empty response

var TurnsHandler = "Gol.TakeTurns"
type Request struct {
	Params Params
}
type Response struct {
	ID int
	Slice [][]uint8 //final strip
	Turn int //to report to distributor events
	Alive []util.Cell //alive cells to report to distributor events
}

var BrokerAliveHandler = "Broker.ReportAlive"
var AliveHandler = "Gol.ReportAlive"
//EmptyRequest
type AliveResponse struct {
	Alive []util.Cell
	OnTurn int
}

var PollWorldHandler = "Gol.PollWorld"
//EmptyRequest
//Response

var FinishHander = "Gol.Finish"
//EmptyRequest
//EmptyResponse

var KillHandler = "Gol.Kill"
//EmptyRequest
//EmptyResponse

var PauseHandler = "Gol.PauseGol"
type PauseRequest struct {
	Pause bool
}
type PauseResponse struct {
    World [][]byte
    Turns int
}

var ClientHandler = "Broker.AcceptClient"
type NewClientRequest struct {
	World [][]byte
	Params Params
}
type NewClientResponse struct {
	World [][]byte
	Turns int
	Alive []util.Cell
}

