/* Copyright (C) 2013 Vincent Petithory <vincent.petithory@gmail.com>
 *
 * This file is part of mpdclient.
 *
 * mpdclient is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * mpdclient is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with mpdclient.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package mpdclient

import (
	"errors"
	"fmt"
)

type ChannelMessage struct {
	Channel string
	Message string
}

func (c *MPDClient) subscriptionCmd(cmd string) *response {
	c.Logger.Println(cmd, "> entering")
	c.idle.MaybeWait()
	var r response

	c.Logger.Println(cmd, "> sending noidle")
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
	c.Logger.Println(cmd, "> sent noidle")
	id, err = c.subscriptionConn.Cmd(cmd)
	if err != nil {
		r.Err = err
		return &r
	}
	c.Logger.Println(cmd, "> sending id", id)
	var req request = request(id)
	c.idle.reqCh <- &req
	c.Logger.Println(cmd, "> sent, waiting response", id)
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
	n := len(res.Data)
	msgs := make([]ChannelMessage, n/2)
	for i := 0; i < n; i += 2 {
		matchC := responseRegexp.FindStringSubmatch(res.Data[i])
		matchM := responseRegexp.FindStringSubmatch(res.Data[i+1])
		if matchC == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i]))
		}
		if matchM == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i+1]))
		}
		var channelMessage *ChannelMessage = &msgs[i/2]
		channelMessage.Channel = matchC[2]
		channelMessage.Message = matchM[2]
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
			c.Logger.Panicf("Panic in subscriptionloop: %s\n", err)
		}
	}()
	for {
		c.Logger.Println("Entering subscriptionloop")
		id, err := c.subscriptionConn.Cmd("idle message")
		if err != nil {
			panic(err)
		}

		c.Logger.Println("subscriptionloop ready1")
		c.subscriptionConn.StartResponse(id)
		c.Logger.Println("subscriptionloop ready2")

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
			c.Logger.Println("subsystem", *subsystem, "changed")
			go c.sendIdleChange(*subsystem)
		} else {
			c.Logger.Println("Noidle triggered")
			select {
			case <-c.idle.quitCh:
				return
			default:
				c.Logger.Println("waiting the request id")
				req := <-c.idle.reqCh
				reqId := uint(*(req))
				c.Logger.Println("got request id", reqId)
				c.subscriptionConn.StartResponse(reqId)
				res := processConnData(c.subscriptionConn)
				c.subscriptionConn.EndResponse(reqId)
				c.idle.resCh <- &res
			}
		}
	}
}
