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
	"sync"
)

type idleState struct {
	c      *sync.Cond
	isIdle bool
	quitCh chan bool
	reqCh  chan *request
	resCh  chan *response
}

func (is *idleState) MaybeWait() {
	if !is.isIdle {
		is.c.L.Lock()
		is.c.Wait()
		is.c.L.Unlock()
	}
}

type idleListener struct {
	Ch         chan string
	active     bool
	subsystems []string
}

func (is *idleListener) Close() {
	close(is.Ch)
	is.active = false
}

func (c *MPDClient) Idle(subsystems ...string) *idleListener {
	is := idleListener{make(chan string), true, subsystems}
	c.idleListeners = append(c.idleListeners, &is)
	return &is
}

func (c *MPDClient) sendIdleChange(subsystem string) {
	for i, idleListener := range c.idleListeners {
		if idleListener.active == true {
			if len(idleListener.subsystems) == 0 {
				idleListener.Ch <- subsystem
			} else {
				for _, wantedSubsystem := range idleListener.subsystems {
					if wantedSubsystem == subsystem {
						c.Logger.Println("sending", subsystem, "to", i)
						idleListener.Ch <- subsystem
					}
				}
			}
		}
	}
}

func (c *MPDClient) idleLoop() {
	defer func() {
		if err := recover(); err != nil {
			c.Logger.Panicf("Panic in idleloop: %s\n", err)
		}
	}()
	for {
		c.Logger.Println("Entering idle mode")
		id, err := c.idleConn.Cmd("idle")
		if err != nil {
			panic(err)
		}

		c.Logger.Println("Idle mode ready1")
		c.idleConn.StartResponse(id)
		c.Logger.Println("Idle mode ready2")

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
			c.Logger.Println("subsystem", *subsystem, "changed")
			go c.sendIdleChange(*subsystem)
		} else {
			c.Logger.Println("Noidle triggered")
			select {
			case <-c.idle.quitCh:
				return
			default:
			}
		}
	}
}
