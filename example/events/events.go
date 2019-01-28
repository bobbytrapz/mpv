package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/bobbytrapz/mpv"
)

const sockfn = "/tmp/mpv-example"

func main() {
	// this is how I imagine one might use this package...

	// create context for shutting down mpv
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start mpv
	client, err := mpv.NewClient(ctx, sockfn)
	if err != nil {
		panic(err)
	}
	defer client.Wait()

	// use client
	go func() {
		movieFn := "some-movie.mp4"
		if err := client.AddFile(movieFn); err != nil {
		}
		<-mpv.FileLoaded
		fmt.Println("[ok] loaded")
	}()

	// handle events
	go func() {
		for {
			select {
			case <-mpv.Seek:
				fmt.Println("[ok] seek!!")
			case <-ctx.Done():
				return
			}
		}
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
