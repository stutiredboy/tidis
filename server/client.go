//
// client.go
// Copyright (C) 2018 YanMing <yming0221@gmail.com>
//
// Distributed under terms of the MIT license.
//

package server

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/YongMan/go/log"
	"github.com/YongMan/tedis/tedis"
	"github.com/siddontang/goredis"
)

var ErrCommand = errors.New("command error")

type Client struct {
	app *App

	tdb *tedis.Tedis

	// request is processing
	cmd  string
	args [][]byte

	buf bytes.Buffer

	conn net.Conn

	rReader *goredis.RespReader
	rWriter *goredis.RespWriter
}

func newClient(app *App) *Client {
	client := &Client{
		app: app,
		tdb: app.tdb,
	}
	return client
}

func ClientHandler(conn net.Conn, app *App) {
	c := newClient(app)

	c.conn = conn
	// connection buffer setting

	br := bufio.NewReader(conn)
	c.rReader = goredis.NewRespReader(br)

	bw := bufio.NewWriter(conn)
	c.rWriter = goredis.NewRespWriter(bw)

	app.clientWG.Add(1)

	go c.connHandler()
}

func (c *Client) connHandler() {

	defer func(c *Client) {
		c.conn.Close()
		c.app.clientWG.Done()
	}(c)

	select {
	case <-c.app.quitCh:
		return
	default:
		break
	}

	for {
		c.cmd = ""
		c.args = nil

		req, err := c.rReader.ParseRequest()
		if err != nil && err != io.EOF {
			log.Error(err.Error())
			return
		} else if err != nil {
			return
		}
		err = c.handleRequest(req)
		if err != nil && err != io.EOF {
			log.Error(err.Error())
			return
		}
	}
}

func (c *Client) handleRequest(req [][]byte) error {
	if len(req) == 0 {
		c.cmd = ""
		c.args = nil
	} else {
		c.cmd = strings.ToLower(string(req[0]))
		c.args = req[1:]
	}
	c.execute()
	return nil
}

func (c *Client) execute() {
	var err error

	start := time.Now()

	if len(c.cmd) == 0 {
		err = ErrCommand
	} else if f, ok := cmdFind(c.cmd); !ok {
		err = ErrCommand
	} else {
		err = f(c)
	}
	// TODO
	if err != nil {
		c.rWriter.WriteError(err)
	}
	c.rWriter.Flush()
	log.Debugf("command time cost %d", time.Now().Sub(start).Nanoseconds())
	return
}
