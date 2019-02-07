# mpv
Package for controlling mpv using JSON IPC

I plan on using it for a different project but there has not been a lot of testing yet. I wrote some examples if you need help.

Check the [mpv reference manual](https://mpv.io/manual/) for a complete reference.

Here's a quick look at how to use it. More detail can be found in the examples directory:

```
// create context for shutting down mpv
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// start new mpv process and connect to it
client, err := mpv.NewClient(ctx, "/tmp/mpv-sock")
if err != nil {
	panic(err)
}
defer client.Close()

// use client to send commands to mpv
if v, err := client.Version(); err == nil {
	fmt.Println("version:", v)
}
// here we stop after we are done with mpv
cancel()
```
