// Package rpc provides a client for making remote procedure calls (RPC) to a server.
// It allows clients to connect to a server and invoke methods on the server using RPC.
// The client can be used to change the log level of the server or perform other commands.
// It uses the net/rpc package to handle the RPC communication and provides a simple interface for clients to interact with the server.
package rpc

import (
	"context"
	"fmt"
	"net/rpc"

	"github.com/ambitiousfew/rxd/log"
)

// Client is a struct that represents an RPC client.
// It contains an RPC client that can be used to make remote procedure calls to a server.
type Client struct {
	client *rpc.Client
}

// NewClientWithPath creates a new RPC client with the specified address and path.
// The address is the server's address, and the path is the RPC endpoint.
// If the path is empty, it defaults to "/rpc".
func NewClientWithPath(addr, path string) (*Client, error) {
	// Create a new RPC client
	client, err := rpc.DialHTTPPath("tcp", addr, path)

	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

// NewClient creates a new RPC client with the specified address.
// The address is the server's address, and it defaults to the "/rpc" endpoint.
func NewClient(addr string) (*Client, error) {
	// Create a new RPC client
	client, err := rpc.DialHTTPPath("tcp", addr, "/rpc")

	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

// ChangeLogLevel changes the log level of the server.
// It takes a context and a log level as parameters.
// The context is used to control the timeout and cancellation of the request.
func (c *Client) ChangeLogLevel(ctx context.Context, level log.Level) error {
	var resp error

	doneC := make(chan *rpc.Call, 1)
	call := c.client.Go("CommandHandler.ChangeLogLevel", level, &resp, doneC)

	select {
	case <-ctx.Done():
		if call != nil {
			call.Done <- call
		}
	case result := <-doneC:
		fmt.Println("result: ", result)
		if result.Error != nil {
			fmt.Println("error: ", result.Error)
			return result.Error
		}
		fmt.Println("resp: ", resp)
		return nil
	}
	return resp
}

// Close closes the RPC client connection.
// It returns an error if the connection could not be closed.
func (c *Client) Close() error {
	return c.client.Close()
}
