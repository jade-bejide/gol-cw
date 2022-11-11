package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var TurnsHandler = "Gol.TakeTurns"
var AliveHandler = "Gol.ReportAlive"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	World [][]uint8 //final world
	Turns int //to report to distributor events
	Alive []util.Cell //alive cells to report to distributor events
}

type Request struct {
	World  [][]uint8
	Params Params
}

type AliveRequest struct {}

type AliveResponse struct {
	Alive int
	OnTurn int
}

