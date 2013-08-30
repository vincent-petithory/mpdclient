package mpdfav

import (
	"errors"
	"fmt"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"log"
)

const (
	network         = "tcp"
	StickerSongType = "song"
)

var uid uint = 1

type ChannelMessage struct {
	Channel string
	Message string
}

type Info map[string]string

type MPDError struct {
	Ack uint
	CommandListNum uint
	CurrentCommand string
	MessageText string
}

func (me MPDError) Error() string {
	return fmt.Sprintf("%d@%d %s: %s", me.Ack, me.CommandListNum, me.CurrentCommand, me.MessageText)
}

var responseRegexp = regexp.MustCompile(`(\w+): (.+)`)
var mpdErrorRegexp = regexp.MustCompile(`ACK \[(\d+)@(\d+)\] {(\w+)} (.+)`)

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
	Host string
	Port uint
	conn *textproto.Conn
	idleConn *textproto.Conn
	subscriptionConn *textproto.Conn
	idle *idleState
	uid uint
	log *log.Logger
}

type idleState struct {
	c *sync.Cond
	isIdle bool
	quitCh chan bool
	reqCh chan *request
	resCh chan *response
	subscriptions []*idleSubscription
}

type request uint

type response struct {
	Data []string
	Err error
	MPDErr *MPDError
}

func (is *idleState) MaybeWait() {
	if !is.isIdle {
		is.c.L.Lock()
		is.c.Wait()
		is.c.L.Unlock()
	}
}

type idleSubscription struct {
	Ch chan string
	active bool
	subsystems []string
}

func (is *idleSubscription) Close() {
	close(is.Ch)
	is.active = false
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

func (c *MPDClient) CurrentSong() (*Info, error) {
	res := c.Cmd("currentsong")
	if res.Err != nil {
		return nil, res.Err
	}
	if res.MPDErr != nil {
		return nil, res.MPDErr
	}
	info := make(Info)
	err := info.Fill(res.Data)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *MPDClient) Status() (*Info, error) {
	res := c.Cmd("status")
	if res.Err != nil {
		return nil, res.Err
	}
	if res.MPDErr != nil {
		return nil, res.MPDErr
	}
	info := make(Info)
	err := info.Fill(res.Data)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *MPDClient) StickerGet(stype, uri, stickerName string) (string, error) {
	res := c.Cmd(fmt.Sprintf(
		"sticker get \"%s\" \"%s\" \"%s\"",
		stype,
		uri,
		stickerName,
	))
	if res.Err != nil {
		return "", res.Err
	}
	if res.MPDErr != nil {
		// If no such sticker, return empty string
		if strings.Index(res.MPDErr.MessageText, "no such sticker") != -1 {
			return "", nil
		}
		return "", res.MPDErr
	}

	match := responseRegexp.FindStringSubmatch(res.Data[0])
	if match == nil {
		return "", errors.New(fmt.Sprintf("Invalid input: %s", res.Data[0]))
	}
	pair := match[2]

	fieldSepIndex := strings.Index(pair, "=")
	if fieldSepIndex == -1 {
		return "", errors.New(fmt.Sprintf("Invalid input: %s", pair))
	}
	stickerVal := pair[fieldSepIndex+1:]

	return stickerVal, nil
}

func (c *MPDClient) StickerSet(stype, uri, stickerName, value string) error {
	res := c.Cmd(fmt.Sprintf(
		"sticker set \"%s\" \"%s\" \"%s\" \"%s\"",
		stype,
		uri,
		stickerName,
		value,
	))
	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}

	return nil
}

func (c *MPDClient) Subscribe(channel string) error {
	res := c.subscriptionCmd(fmt.Sprintf(
		"subscribe \"%s\"",
		channel,
	))
	if res.MPDErr != nil {
		return res.MPDErr
	}
	return nil
}

func (c *MPDClient) Unsubscribe(channel string) error {
	res := c.subscriptionCmd(fmt.Sprintf(
		"unsubscribe \"%s\"",
		channel,
	))
	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}
	return nil
}

func (c *MPDClient) ReadMessages() ([]ChannelMessage, error) {
	res := c.subscriptionCmd("readmessages")
	if res.Err != nil {
		return nil, res.Err
	}
	if res.MPDErr != nil {
		return nil, res.MPDErr
	}
	msgs := make([]ChannelMessage, 0)
	n := len(res.Data)
	for i := 0; i < n; i += 2 {
		matchC := responseRegexp.FindStringSubmatch(res.Data[i])
		matchM := responseRegexp.FindStringSubmatch(res.Data[i+1])
		if matchC == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i]))
		}
		if matchM == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i+1]))
		}
		msgs = append(msgs, ChannelMessage{matchC[2], matchM[2]})
	}
	return msgs, nil
}

func (c *MPDClient) Channels() ([]string, error) {
	res := c.Cmd("channels")
	if res.Err != nil {
		return nil, res.Err
	}
	if res.MPDErr != nil {
		return nil, res.MPDErr
	}

	channels := make([]string, 0)
	for _, line := range res.Data {
		match := responseRegexp.FindStringSubmatch(line)
		if match == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", line))
		}
		if match[1] != "channel" {
			return nil, errors.New(fmt.Sprintf("Unexpected keyt: %s", match[1]))
		}
		channels = append(channels, match[2])
	}
	return channels, nil
}

func (c *MPDClient) SendMessage(channel, text string) error {
	res := c.Cmd(fmt.Sprintf(
		"sendmessage \"%s\" \"%s\"",
		channel,
		text,
	))

	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}
	return nil
}

func (c *MPDClient) Idle(subsystems ...string) *idleSubscription {
	is := idleSubscription{make(chan string), true, subsystems}
	c.idle.subscriptions = append(c.idle.subscriptions, &is)
	return &is
}

func (c *MPDClient) sendSubscriptionsFor(subsystem string) {
	for i, subscription := range c.idle.subscriptions {
		if subscription.active == true {
			if len(subscription.subsystems) == 0 {
				subscription.Ch <- subsystem
			} else {
				for _, wantedSubsystem := range subscription.subsystems {
					if wantedSubsystem == subsystem {
						c.log.Println("sending", subsystem, "to", i)
						subscription.Ch <- subsystem
					}
				}
			}
		}
	}
}

func (c *MPDClient) idleloop() {
	defer func() {
        if err := recover(); err != nil {
            c.log.Println("Panic in idleloop:", err)
        }
	}()
        for {
			c.log.Println("Entering idle mode")
			id, err := c.idleConn.Cmd("idle")
			if err != nil {
				panic(err)
			}

			c.log.Println("Idle mode ready1")
			c.idleConn.StartResponse(id)
			c.log.Println("Idle mode ready2")

			var subsystem *string
			var idleErr error
			for {
				line, idleErr := c.idleConn.ReadLine()
				if idleErr != nil {

					break
				}
				if line == "OK" {
					break
				} else {
					match := responseRegexp.FindStringSubmatch(line)
					if match == nil {
						break
					}
					key := match[1]
					if key == "changed" {
						l := match[2]
						subsystem = &l
					}
				}
			}

			c.idleConn.EndResponse(id)
			if idleErr != nil {
				panic(idleErr)
			}

			if subsystem != nil {
				c.log.Println("subsystem", *subsystem, "changed")
				go c.sendSubscriptionsFor(*subsystem)
			} else {
				c.log.Println("Noidle triggered")
				select {
				case <-c.idle.quitCh:
					return
				default:
				}
			}
	}
}

func (c *MPDClient) subscriptionloop() {
	defer func() {
        if err := recover(); err != nil {
            c.log.Println("Panic in subscriptionloop:", err)
        }
	}()
        for {
			c.log.Println("Entering subscriptionloop")
			id, err := c.subscriptionConn.Cmd("idle message")
			if err != nil {
				panic(err)
			}

			c.log.Println("subscriptionloop ready1")
			c.subscriptionConn.StartResponse(id)
			c.log.Println("subscriptionloop ready2")

			// Signal other goroutines that idle mode is ready
			c.idle.c.L.Lock()
			c.idle.c.Signal()
			c.idle.c.L.Unlock()
			c.idle.isIdle = true

			var subsystem *string
			var idleErr error
			for {
				line, idleErr := c.subscriptionConn.ReadLine()
				if idleErr != nil {

					break
				}
				if line == "OK" {
					break
				} else {
					match := responseRegexp.FindStringSubmatch(line)
					if match == nil {
						break
					}
					key := match[1]
					if key == "changed" {
						l := match[2]
						subsystem = &l
					}
				}
			}

			c.subscriptionConn.EndResponse(id)
			c.idle.isIdle = false
			if idleErr != nil {
				panic(idleErr)
			}

			if subsystem != nil {
				c.log.Println("subsystem", *subsystem, "changed")
				go c.sendSubscriptionsFor(*subsystem)
			} else {
				c.log.Println("Noidle triggered")
				select {
				case <-c.idle.quitCh:
					return
				default:
					c.log.Println("waiting the request id")
					req := <-c.idle.reqCh
					reqId := uint(*(req))
					c.log.Println("got request id", reqId)
					c.subscriptionConn.StartResponse(reqId)
					res := processConnData(c.subscriptionConn)
					c.subscriptionConn.EndResponse(reqId)
					c.idle.resCh <- &res
				}
			}
	}
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

func (c *MPDClient) subscriptionCmd(cmd string) *response {
	c.log.Println(cmd, "> entering")
	c.idle.MaybeWait()
	var r response

	c.log.Println(cmd, "> sending noidle")
	id, err := c.subscriptionConn.Cmd("noidle")
	if err != nil {
		r.Err = err
		return &r
	}
	c.subscriptionConn.StartResponse(id)
	c.subscriptionConn.EndResponse(id)

	if err != nil {
		r.Err = err
		return &r
	}
	c.log.Println(cmd, "> sent noidle")
	id, err = c.subscriptionConn.Cmd(cmd)
	if err != nil {
		r.Err = err
		return &r
	}
	c.log.Println(cmd, "> sending id", id)
	var req request = request(id)
	c.idle.reqCh <- &req
	c.log.Println(cmd, "> sent, waiting response", id)
	res := <- c.idle.resCh
	return res
}

func (c *MPDClient) Close() error {
	// Shut down idle mode
	c.log.Println("sending quit command")
	c.idle.MaybeWait()
	go func() {
		c.idle.quitCh<-true
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
		c.idle.quitCh<-true
	}()

	id, err = c.idleConn.Cmd("noidle")
	if err != nil {
		return err
	}
	c.idleConn.StartResponse(id)
	c.idleConn.EndResponse(id)
	CloseConn(c.idleConn)

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

func newConn(host string, port uint) (*textproto.Conn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := textproto.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	line, err := conn.ReadLine()
	if err != nil {
		return nil, err
	}

	if line[0:6] != "OK MPD" {
		return nil, errors.New("MPD: not OK")
	}
	return conn, nil
}

func Connect(host string, port uint) (*MPDClient, error) {
	conn, err := newConn(host, port)
	if err != nil {
		return nil, err
	}
	idleConn, err := newConn(host, port)
	if err != nil {
		return nil, err
	}
	subscriptionConn, err := newConn(host, port)
	if err != nil {
		return nil, err
	}

	mpdcLog, err := newMPDCLogger(uid, true)
	if err != nil {
		return nil, err
	}

		var m sync.Mutex
    c := sync.NewCond(&m)
	idleState := &idleState{c, false, make(chan bool), make(chan *request), make(chan *response), []*idleSubscription{}}

	mpdc := &MPDClient{host, port, conn, idleConn, subscriptionConn, idleState, uid, mpdcLog}
	uid++
	go mpdc.idleloop()
	go mpdc.subscriptionloop()
	return mpdc, nil
}
