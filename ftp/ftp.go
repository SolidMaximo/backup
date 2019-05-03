// inspired by https://github.com/jum/tinyftp
// which is copyright 2013 Jens-Uwe Mager jum@anubis.han.de
package ftp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Connection struct {
	c     net.Conn
	conn  *textproto.Conn
	addr  string
	port  string
	Debug bool
}

func NewConnection(addr string, port string) *Connection {
	var c Connection
	c.addr = addr
	c.port = port
	return &c
}

func (c *Connection) debug(v ...interface{}) {
	if c.Debug {
		log.Println(v...)
	}
}

func (c *Connection) debugf(format string, v ...interface{}) {
	if c.Debug {
		log.Printf(format, v...)
	}
}

func (c *Connection) debugResponse(code int, msg string) {
	if c.Debug {
		log.Printf("code: %d, msg: %v\n", code, msg)
	}
}

func (c *Connection) Connect() error {
	c.debugf("Connecting to %s", c.addr+":"+c.port)
	conn, err := net.DialTimeout("tcp", c.addr+":"+c.port, 5*time.Second)
	if err != nil {
		return err
	}
	c.c = conn

	c.conn = textproto.NewConn(conn)
	code, msg, err := c.conn.ReadResponse(2)
	c.debugResponse(code, msg)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) Cmd(exp int, format string, args ...interface{}) (code int, msg string, err error) {
	c.debugf(fmt.Sprintf("Cmd %d", exp)+format, args...)
	err = c.conn.PrintfLine(format, args...)
	if err != nil {
		return
	}

	return c.conn.ReadResponse(exp)
}

func (c *Connection) Quit() error {
	code, msg, err := c.Cmd(221, "QUIT")
	c.debugResponse(code, msg)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) Type(t string) error {
	code, msg, err := c.Cmd(200, "TYPE %s", t)
	c.debugResponse(code, msg)
	if err != nil {
		return err
	}

	return nil
}

var passiveRegexp = regexp.MustCompile(`([\d]+),([\d]+),([\d]+),([\d]+),([\d]+),([\d]+)`)

func (c *Connection) Passive() (net.Conn, error) {
	code, msg, err := c.Cmd(227, "PASV")
	c.debugResponse(code, msg)
	if err != nil {
		return nil, err
	}

	matches := passiveRegexp.FindStringSubmatch(msg)
	if matches == nil {
		return nil, errors.New("Cannot parse PASV response: " + msg)
	}

	ph, err := strconv.Atoi(matches[5])
	if err != nil {
		return nil, err
	}

	pl, err := strconv.Atoi(matches[6])
	if err != nil {
		return nil, err
	}

	port := strconv.Itoa((ph << 8) | pl)
	addr := strings.Join(matches[1:5], ".") + ":" + port

	timeout := 10 * time.Second
	dconn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}

	return dconn, nil
}

// todo: support argument to namelist e.g. *.ls
func (c *Connection) NameList() ([]string, error) {
	dconn, err := c.Passive()
	if err != nil {
		return nil, err
	}
	defer dconn.Close()

	code, msg, err := c.Cmd(1, "NLST")
	c.debugResponse(code, msg)
	if err != nil {
		return nil, err
	}

	var files []string
	scanner := bufio.NewScanner(dconn)
	c.debug("Getting list of files...")
	for scanner.Scan() {
		files = append(files, scanner.Text())
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}
	dconn.Close()

	c.debugf("Received list of %d files\n", len(files))
	c.debug("Waiting for response from main connection...")

	code, msg, err = c.conn.ReadResponse(226)
	c.debugResponse(code, msg)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (c *Connection) Download(filename string, dest string) error {
	c.debugf("creating local file %s\n", dest+"/"+filename)
	f, err := os.Create(dest + "/" + filename)
	if err != nil {
		return err
	}
	defer f.Close()

	c.debugf("creating writer %s\n", filename)
	w := bufio.NewWriter(f)
	defer w.Flush()

	c.debugf("getting passive connection %s\n", filename)
	dconn, err := c.Passive()
	if err != nil {
		return err
	}
	defer dconn.Close()

	c.debugf("issuing RETR cmd %s\n", filename)
	code, msg, err := c.Cmd(1, "RETR %s", filename)
	c.debugf("got response for RETR %s\n", filename)
	if err != nil {
		return err
	}

	c.debugf("copying %s\n", filename)
	_, err = io.Copy(w, dconn)
	if err != nil {
		return err
	}
	dconn.Close()

	c.debugf("reading response %s\n", filename)
	code, msg, err = c.conn.ReadResponse(2)
	c.debugResponse(code, msg)
	if err != nil {
		return err
	}

	return nil
}
