package main

import (
	"context"
	"log"
	"time"

	"os"

	rxlog "github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/pkg/rpc"
)

func processCommand(command string) rpc.Command {
	switch command {
	case "setlevel":
		return rpc.SetLevel
	// case "stop":
	// 	return rpc.Stop
	// case "start":
	// 	return rpc.Start
	// case "list":
	// 	return rpc.List
	default:
		return rpc.Unknown
	}
}

func main() {
	// should never take more than a few seconds to run this client.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if len(os.Args) < 2 {
		log.Println("usage: rpc_client setlevel <command>")
		os.Exit(1)
	}

	cmd := processCommand(os.Args[1])

	if cmd == rpc.Unknown {
		log.Println("unknown command:", os.Args[1])
		os.Exit(1)
	}

	// Create a new RPC client
	client, err := rpc.NewClientWithPath("localhost:1337", "/rpc")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	switch cmd {
	case rpc.SetLevel:
		if len(os.Args) < 3 {
			log.Println("usage: rpc_client loglevel <level>")
			os.Exit(1)
		}

		level := int8(rxlog.LevelFromString(os.Args[2]))

		if level < 0 {
			log.Println("unknown log level:", os.Args[2])
			os.Exit(1)
		}

		// Call the ChangeLogLevel method on the RPC server, pass level and pass response
		err = client.ChangeLogLevel(ctx, rxlog.Level(level))
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}

		log.Println("log level changed to:", os.Args[2])
		return
	}

	log.Println("client has exited successfully.")
}
