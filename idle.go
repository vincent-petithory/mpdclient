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
	subscriptions []*idleSubscription
}

func (is *idleState) MaybeWait() {
	if !is.isIdle {
		is.c.L.Lock()
		is.c.Wait()
		is.c.L.Unlock()
	}
}

type idleSubscription struct {
	Ch         chan string
	active     bool
	subsystems []string
}

func (is *idleSubscription) Close() {
	close(is.Ch)
	is.active = false
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
