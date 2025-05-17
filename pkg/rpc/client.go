package rpc

import (
	"context"
	"fmt"
	"net/rpc"

	"github.com/ambitiousfew/rxd/v2/log"
)

type Client struct {
	client *rpc.Client
}

func NewClientWithPath(addr, path string) (*Client, error) {
	// Create a new RPC client
	client, err := rpc.DialHTTPPath("tcp", addr, path)

	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

func NewClient(addr string) (*Client, error) {
	// Create a new RPC client
	client, err := rpc.DialHTTPPath("tcp", addr, "/rpc")

	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

func (c *Client) ChangeLogLevel(ctx context.Context, level log.Level) error {
	var resp error

	doneC := make(chan *rpc.Call, 1)
	call := c.client.Go("RPCCommandHandler.ChangeLogLevel", level, &resp, doneC)

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

func (c *Client) SendSignal(ctx context.Context, signal CommandSignal, service string) error {
	var resp error

	payload := CommandSignalPayload{
		Service: service,
		Signal:  signal,
	}

	doneC := make(chan *rpc.Call, 1)
	call := c.client.Go("RPCCommandHandler.SendCommandSignal", payload, &resp, doneC)

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

func (c *Client) Close() error {
	return c.client.Close()
}
