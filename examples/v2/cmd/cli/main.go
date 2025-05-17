package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ambitiousfew/rxd/v2/log"
	"github.com/ambitiousfew/rxd/v2/pkg/rpc"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)
		<-signalC
		signal.Stop(signalC)
		cancel()
	}()

	// Usage: ./<binary> <command> <arg1> <arg2> ...
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"help"}
	}

	// Usage: ./<binary> <help|version>
	switch args[0] {
	case "help":
		fmt.Printf("Usage: %s <command>\n", os.Args[0])
		return // no need to connect to the rpc server
	case "version":
		fmt.Println("v0.0.1")
		return // no need to connect to the rpc server
	default:
	}

	client, err := rpc.NewClient("localhost:8080")
	if err != nil {
		fmt.Println("Failed to connect to the rpc server:", err)
		return
	}
	defer client.Close()
	switch args[0] {
	case "signal":
		// args: [<signal> <signal> <service>]
		if len(args) < 3 {
			fmt.Println("Usage: signal <signal> <service>")
			return
		}

		signal := rpc.CommandSignalFromString(args[2])
		if signal == rpc.CommandSignalUnknown {
			fmt.Println("Unknown service signal:", args[2])
			return
		}

		err = client.SendSignal(ctx, signal, args[1])
		if err != nil {
			fmt.Println("Failed to signal service:", err)
			return
		}

		fmt.Println("Service signaled:", signal)

	case "loglevel":
		if len(args) < 2 {
			fmt.Println("Usage: loglevel <level>")
			return
		}

		lvl := log.LevelFromString(args[1])
		if lvl == log.LevelEmergency {
			fmt.Println("Unknown log level:", args[1])
			return
		}

		err = client.ChangeLogLevel(ctx, lvl)
		if err != nil {
			fmt.Println("Failed to change log level:", err)
			return
		}

	default:
		fmt.Println("Unknown command:", args[0])
	}
}
