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


var AliveHandler = "Gol.ReportAlive"
//EmptyRequest
type AliveResponse struct {
	Alive int
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

type NewClientRequest struct {
	World [][]byte
	Params Params
}

type NewClientResponse struct {
	World [][]byte
}

