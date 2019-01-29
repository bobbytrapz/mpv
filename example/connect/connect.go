package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/bobbytrapz/mpv"
)

import "log"

const sockfn = "/tmp/mpv-example"

func main() {
	// this is how I imagine one might use this package...

	mpv.Log = log.Printf

	// create context for shutting down mpv
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect to an existing mpv socket
	// run: mpv --input-unix-socket=/tmp/mpv-example --idle=yes
	client, err := mpv.NewClientConnect(ctx, sockfn)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// use client
	go func() {
		movieFn := "some-movie.mp4"
		if err := client.PlayFile(movieFn); err != nil {
		}
		<-mpv.FileLoaded
		fmt.Println("[ok] loaded")
	}()

	// handle interrupt
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	for {
		select {
		case <-ctx.Done():
			cancel()
			return
		case <-sig:
			cancel()
			return
		case <-mpv.Closed:
			cancel()
			return
		}
	}
}
