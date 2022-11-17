package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type EmptyRequest struct {}
type EmptyResponse struct{}

var TurnsHandler = "Gol.TakeTurns"
type Request struct {
	World  [][]uint8
	Params Params
}
type Response struct {
	World [][]uint8 //final world
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

