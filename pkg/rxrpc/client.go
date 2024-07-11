package rxrpc

import (
	"net/rpc"

	"github.com/ambitiousfew/rxd/log"
)

type Client struct {
	c *rpc.Client
}

func NewClientWithPath(addr, path string) (*Client, error) {
	// Create a new RPC client
	client, err := rpc.DialHTTPPath("tcp", addr, path)

	if err != nil {
		return nil, err
	}

	return &Client{c: client}, nil
}

func NewClient(addr string) (*Client, error) {
	// Create a new RPC client
	client, err := rpc.DialHTTPPath("tcp", addr, "/rpc")

	if err != nil {
		return nil, err
	}

	return &Client{c: client}, nil
}

func (c *Client) ChangeLogLevel(level log.Level) error {
	var resp error
	err := c.c.Call("CommandHandler.ChangeLogLevel", level, &resp)
	if err != nil {
		return err
	}
	return resp
}

func (c *Client) Close() error {
	return c.c.Close()
}
