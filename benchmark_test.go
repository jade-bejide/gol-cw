package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

func BenchmarkGolWorkers(b *testing.B) {
	os.Stdout = nil

	var params gol.Params
	params.ImageWidth = 512
	params.ImageHeight = 512
	params.Turns = 10
	params.Turns = 5000


	////keyPresses := make(chan rune, 10)
	//events := make(chan gol.Event, 1000)

	//go gol.Run(params, events, keyPresses)

	//benchmark thread by thread
	for threads := 2; threads <= 8; threads++ {
		b.Run(fmt.Sprintf("%d_workers", threads), func(b *testing.B) {
			//b.ResetTimer()
			//b.StartTimer()
			params.Threads = threads
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event, 1000)
				go gol.Run(params, events, nil, false)
				for range events {

				}
			}
		})
	}

}