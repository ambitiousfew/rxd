package main

import (
	"context"
	"log"
	"time"

	"os"

	rxlog "github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/pkg/rxrpc"
)

func processCommand(command string) rxrpc.Command {
	switch command {
	case "setlevel":
		return rxrpc.SetLevel
	// case "stop":
	// 	return rpc.Stop
	// case "start":
	// 	return rpc.Start
	// case "list":
	// 	return rpc.List
	default:
		return rxrpc.Unknown
	}
}

func main() {
	// should never take more than 10 seconds to run this client.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = ctx

	if len(os.Args) < 2 {
		log.Println("usage: rpc_client setlevel <command>")
		os.Exit(1)
	}

	cmd := processCommand(os.Args[1])

	if cmd == rxrpc.Unknown {
		log.Println("unknown command:", os.Args[1])
		os.Exit(1)
	}

	// Create a new RPC client
	client, err := rxrpc.NewClientWithPath("192.168.0.210:1337", "/rpc")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	switch cmd {
	case rxrpc.SetLevel:
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
		err = client.ChangeLogLevel(rxlog.Level(level))
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}

		log.Println("log level changed to:", os.Args[2])
		return
	}

	// // Call the Echo method on the RPC server, pass payload and pass response
	// resp, err := client.SendCommand(payload)
	// if err != nil {
	// 	log.Println(err)
	// 	os.Exit(1)
	// }

	// log.Println("command '"+resp.Command.String()+"' succeeded:", resp.Success)

	// if payload.Command == rpc.List {
	// 	log.Println("List of services:", string(resp.Body))
	// }

	// // Close the client
	// err = client.Close()
	// if err != nil {
	// 	log.Println(err)
	// 	os.Exit(1)
	// }

	// log.Println("client has exited successfully.")
}
