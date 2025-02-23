package main

import (
	"context"
	"errors"
	"log"
	"time"

	"os"

	rxlog "github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/pkg/rpc"
)

func main() {
	// should never take more than a few seconds to run this client.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if len(os.Args) < 2 {
		log.Printf("usage: %s help", os.Args[0])
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "help":
		log.Println("usage: rpc_client <command> <args>")
		log.Println("commands:")
		log.Println("  help")
		log.Println("  version")
		log.Println("  signal <signal> <service>")
		log.Println("  loglevel <level>")
		os.Exit(0)
	case "version":
		log.Println("v0.0.1")
		os.Exit(0)
	case "signal":
		if len(os.Args) < 4 {
			log.Println("usage: rpc_client signal <signal> <service>")
			os.Exit(1)
		}
	case "loglevel":
		if len(os.Args) < 3 {
			log.Println("usage: rpc_client loglevel <level>")
			os.Exit(1)
		}
	default:
		log.Println("unknown command:", os.Args[1])
		os.Exit(1)
	}

	// Create a new RPC client
	client, err := rpc.NewClientWithPath("localhost:1337", "/rpc")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer client.Close()

	switch cmd {
	case "loglevel":
		err := handleLogLevel(ctx, client, os.Args[2:])
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
	case "signal":
		err := handleSignal(ctx, client, os.Args[2:])
		if err != nil {
			log.Println("BUTT BALLS: ", err)
			os.Exit(1)
		}
	}

}

func handleLogLevel(ctx context.Context, client *rpc.Client, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: rpc_client loglevel <level>")
	}

	level := int8(rxlog.LevelFromString(args[0]))
	if level < 0 {
		return errors.New("unknown log level: '" + args[0] + "'")
	}

	// Call the ChangeLogLevel method on the RPC server, pass level and pass response
	err := client.ChangeLogLevel(ctx, rxlog.Level(level))
	if err != nil {
		return err
	}

	log.Println("log level changed to:", args[0])
	return nil
}

func handleSignal(ctx context.Context, client *rpc.Client, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: rpc_client signal <signal> <service>")
	}
	signal := rpc.CommandSignalFromString(args[0])
	// Call the ChangeService method on the RPC server, pass service and pass response
	return client.SendSignal(ctx, signal, args[1])
}
