package mpdclient

import (
	"errors"
	"fmt"
	"log"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const network = "tcp"

var uid uint = 1

var responseRegexp = regexp.MustCompile(`(\w+): (.+)`)
var mpdErrorRegexp = regexp.MustCompile(`ACK \[(\d+)@(\d+)\] {(\w+)} (.+)`)
var mpdVersionRegexp = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

type Info map[string]string

func (i *Info) Progress() (int, int) {
	if t, ok := (*i)["time"]; ok {
		fieldSepIndex := strings.Index(t, ":")
		current, err := strconv.ParseFloat(t[0:fieldSepIndex], 0)
		total, err := strconv.ParseFloat(t[fieldSepIndex+1:], 0)
		if err != nil {
			return 0, 0
		}
		return int(current), int(total)
	}
	return 0, 0
}

func (info *Info) AddInfo(data string) error {
	match := responseRegexp.FindStringSubmatch(data)
	if match == nil {
		return errors.New(fmt.Sprintf("Invalid input: %s", data))
	}
	key := match[1]
	val := match[2]
	(*info)[key] = val
	return nil
}

func (i *Info) Fill(data []string) error {
	for _, line := range data {
		err := i.AddInfo(line)
		if err != nil {
			return err
		}
	}
	return nil
}

type MPDClient struct {
	Host             string
	Port             uint
	ProtocolVersion  Version
	conn             *textproto.Conn
	idleConn         *textproto.Conn
	subscriptionConn *textproto.Conn
	pingLoopCh       chan bool
	idle             *idleState
	idleListeners    []*idleListener
	uid              uint
	log              *log.Logger
}

type Version struct {
	Major    uint
	Minor    uint
	Revision uint
}

type request uint

type response struct {
	Data   []string
	Err    error
	MPDErr *MPDError
}

type MPDError struct {
	Ack            uint
	CommandListNum uint
	CurrentCommand string
	MessageText    string
}

func (me MPDError) Error() string {
	return fmt.Sprintf("%d@%d %s: %s", me.Ack, me.CommandListNum, me.CurrentCommand, me.MessageText)
}

func (c *MPDClient) pingLoop() {
	for {
		select {
		case <-c.pingLoopCh:
			return
		case <-time.After(15 * time.Second):
			time.Sleep(15 * time.Second)
			err := c.Ping()
			if err != nil {
				c.log.Println(err)
			} else {
				c.log.Println("PING OK")
			}
		}
	}
}

func processConnData(conn *textproto.Conn) response {
	res := response{Data: make([]string, 0)}
	for {
		line, err := conn.ReadLine()
		if err != nil {
			res.Err = err
			break
		}
		if line == "OK" {
			break
		}
		match := mpdErrorRegexp.FindStringSubmatch(line)
		if match != nil {
			ack, err := strconv.ParseUint(match[1], 0, 0)
			if err != nil {
				res.Err = err
				break
			}
			cln, err := strconv.ParseUint(match[2], 0, 0)
			if err != nil {
				res.Err = err
				break
			}
			res.MPDErr = &MPDError{uint(ack), uint(cln), match[3], match[4]}
			break
		}
		res.Data = append(res.Data, line)
	}
	return res
}

func (c *MPDClient) Cmd(cmd string) *response {
	var r response
	id, err := c.conn.Cmd(cmd)
	if err != nil {
		r.Err = err
		return &r
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)
	res := processConnData(c.conn)
	return &res
}

func (c *MPDClient) Close() error {
	// Shut down idle mode
	c.log.Println("sending quit command")
	c.idle.MaybeWait()
	go func() {
		c.idle.quitCh <- true
	}()

	id, err := c.subscriptionConn.Cmd("noidle")
	if err != nil {
		return err
	}
	c.subscriptionConn.StartResponse(id)
	c.subscriptionConn.EndResponse(id)
	CloseConn(c.subscriptionConn)

	c.log.Println("sending quit command")
	go func() {
		c.idle.quitCh <- true
	}()

	id, err = c.idleConn.Cmd("noidle")
	if err != nil {
		return err
	}
	c.idleConn.StartResponse(id)
	c.idleConn.EndResponse(id)
	CloseConn(c.idleConn)

	// Stop ping loop
	close(c.pingLoopCh)
	// Close connections properly
	CloseConn(c.conn)
	return nil
}

func CloseConn(conn *textproto.Conn) error {
	id, err := conn.Cmd("close")
	if err != nil {
		return err
	}
	conn.StartResponse(id)
	conn.EndResponse(id)
	err = conn.Close()
	if err != nil {
		return err
	}
	return nil
}

func newConn(host string, port uint) (*textproto.Conn, *Version, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := textproto.Dial(network, addr)
	if err != nil {
		return nil, nil, err
	}
	line, err := conn.ReadLine()
	if err != nil {
		return nil, nil, err
	}

	if line[0:6] != "OK MPD" {
		return nil, nil, errors.New("MPD: not OK")
	}
	m := mpdVersionRegexp.FindStringSubmatch(line)
	if m == nil {
		return conn, nil, errors.New("Unknown MPD protocol version")
	}
	mjr, _ := strconv.ParseUint(m[1], 0, 0)
	mnr, _ := strconv.ParseUint(m[2], 0, 0)
	rev, _ := strconv.ParseUint(m[3], 0, 0)
	version := Version{uint(mjr), uint(mnr), uint(rev)}

	return conn, &version, nil
}

func Connect(host string, port uint) (*MPDClient, error) {
	conn, version, err := newConn(host, port)
	if err != nil {
		return nil, err
	}
	idleConn, _, err := newConn(host, port)
	if err != nil {
		return nil, err
	}
	subscriptionConn, _, err := newConn(host, port)
	if err != nil {
		return nil, err
	}

	mpdcLog, err := newMPDCLogger(uid, true)
	if err != nil {
		return nil, err
	}

	var m sync.Mutex
	c := sync.NewCond(&m)
	idleState := &idleState{c, false, make(chan bool), make(chan *request), make(chan *response)}

	mpdc := &MPDClient{host, port, *version, conn, idleConn, subscriptionConn, make(chan bool), idleState, []*idleListener{}, uid, mpdcLog}
	uid++
	go mpdc.pingLoop()
	go mpdc.idleLoop()
	go mpdc.subscriptionLoop()
	return mpdc, nil
}
