package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bobbytrapz/mpv"
)

const sockfn = "/tmp/mpv-example"

func main() {
	// this is how I imagine one might use this package...

	// turn on logging if we'd like
	mpv.Log = log.Printf

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
		if v, err := client.Version(); err == nil {
			fmt.Println("version:", v)
		}
		// here we stop after we are done with mpv
		cancel()
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
