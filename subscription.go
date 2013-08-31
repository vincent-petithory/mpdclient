package mpdfav

import (
	"fmt"
	"errors"
)

type ChannelMessage struct {
	Channel string
	Message string
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
	res := <-c.idle.resCh
	return res
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

func (c *MPDClient) subscriptionLoop() {
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