package stubs

import "uk.ac.bris.cs/gameoflife/util"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Slice struct {
	From int //y coordinates
	To int
}

type EmptyRequest struct {}
type EmptyResponse struct{}


var SetupHandler = "Gol.Setup"
type SetupRequest struct {
	ID int
	Slice Slice
	Params Params
	World [][]byte
}
type SetupResponse struct {
	ID int
	Slice Slice //identify yourselves
}

var TurnHandler = "Gol.TakeTurn"
type Request struct {
	World [][]uint8 //whole world
}
type Response struct {
	ID int
	Strip [][]uint8 //final strip
	Slice Slice
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

var SaveWorldHandler = "Broker.SaveWorld"
type WorldResponse struct {
	World [][]byte
	OnTurn int
}
var PollWorldHandler = "Gol.PollWorld"
//EmptyRequest
//Response

var BrokerFinishHander = "Broker.Finish"

type QuitWorldResponse struct {
	OnTurn int
	Alive [] util.Cell
}
var FinishHander = "Gol.Finish"
//EmptyRequest
//EmptyResponse

var KillHandler = "Gol.Kill"
//EmptyRequest
//EmptyResponse

var BrokerPauseHandler = "Broker.PauseGol"
var PauseHandler = "Gol.PauseGol"
type PauseRequest struct {
	Pause bool
}
type PauseResponse struct {
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

