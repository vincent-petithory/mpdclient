package mpdfav

import (
	"sync"
)

type idleState struct {
	c             *sync.Cond
	isIdle        bool
	quitCh        chan bool
	reqCh         chan *request
	resCh         chan *response
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
						c.log.Println("sending", subsystem, "to", i)
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
			go c.sendIdleChange(*subsystem)
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
