package mpdfav

import (
	"errors"
	"fmt"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
)

const (
	network         = "tcp"
	infoFieldSep    = ": "
	StickerSongType = "song"
)

type ChannelMessage struct {
	Channel string
	Message string
}

type Info map[string]string

var stickerGetRegexp = regexp.MustCompile("sticker: (.+)=(.*)")
var channelMessageRegexp = regexp.MustCompile("channel: (.+)\nmessage: (.+)")

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
	fieldSepIndex := strings.Index(data, infoFieldSep)
	if fieldSepIndex == -1 {
		return errors.New(fmt.Sprintf("Invalid input: %s", data))
	}
	key := data[0:fieldSepIndex]
	(*info)[key] = data[fieldSepIndex+len(infoFieldSep):]
	return nil
}

type MPDClient struct {
	Host string
	Port uint
	conn *textproto.Conn
}

func isMPDError(line string) bool {
	return strings.Index(line, "ACK") != -1
}

func fillInfoUntilOK(c *MPDClient, info *Info) error {
	for {
		line, err := c.conn.ReadLine()
		if err != nil {
			return err
		}
		if line == "OK" {
			break
		}
		err = info.AddInfo(line)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *MPDClient) CurrentSong() (Info, error) {
	id, err := c.conn.Cmd("currentsong")
	if err != nil {
		return nil, err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	info := make(Info)
	err = fillInfoUntilOK(c, &info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (c *MPDClient) Status() (Info, error) {
	id, err := c.conn.Cmd("status")
	if err != nil {
		return nil, err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	info := make(Info)
	err = fillInfoUntilOK(c, &info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (c *MPDClient) StickerGet(stype, uri, stickerName string) (string, error) {
	id, err := c.conn.Cmd(fmt.Sprintf(
		"sticker get \"%s\" \"%s\" \"%s\"",
		stype,
		uri,
		stickerName,
	))

	if err != nil {
		return "", err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	line, err := c.conn.ReadLine()
	match := stickerGetRegexp.FindStringSubmatch(line)
	if match == nil {
		if !isMPDError(line) {
			return "", nil
		}
		// If we found the song but no sticker, return empty string
		if strings.Index(line, "no such sticker") != -1 {
			return "", nil
		}
		return "", errors.New("StickerGet: " + line)
	}
	value := match[2]
	// OK line appears if it's ok,
	// otherwise only one line with error
	okLine, err := c.conn.ReadLine()
	if err != nil {
		return "", err
	}
	if okLine != "OK" {
		return "", errors.New("StickerGet didn't receive OK line: " + okLine)
	}
	return value, nil
}

func (c *MPDClient) StickerSet(stype, uri, stickerName, value string) error {
	id, err := c.conn.Cmd(fmt.Sprintf(
		"sticker set \"%s\" \"%s\" \"%s\" \"%s\"",
		stype,
		uri,
		stickerName,
		value,
	))
	if err != nil {
		return err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	line, err := c.conn.ReadLine()
	if line != "OK" {
		return errors.New("StickerSet: " + line)
	}

	return nil
}

func (c *MPDClient) Subscribe(channel string) error {
	id, err := c.conn.Cmd(fmt.Sprintf(
		"subscribe \"%s\"",
		channel,
	))
	if err != nil {
		return err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	line, err := c.conn.ReadLine()
	if line != "OK" {
		return errors.New("Subscribe: " + line)
	}

	return nil
}

func (c *MPDClient) Unsubscribe(channel string) error {
	id, err := c.conn.Cmd(fmt.Sprintf(
		"unsubscribe \"%s\"",
		channel,
	))
	if err != nil {
		return err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	line, err := c.conn.ReadLine()
	if line != "OK" {
		return errors.New("Unsubscribe: " + line)
	}

	return nil
}

func (c *MPDClient) ReadMessages() ([]ChannelMessage, error) {
	id, err := c.conn.Cmd("readmessages")
	if err != nil {
		return nil, err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	msgs := make([]ChannelMessage, 0)
	for {
		channelOrOkline, err := c.conn.ReadLine()
		if err != nil {
			return msgs, err
		}
		if channelOrOkline == "OK" {
			return msgs, nil
		}
		messageLine, err := c.conn.ReadLine()
		match := channelMessageRegexp.FindStringSubmatch(fmt.Sprintf(`%s
%s`, channelOrOkline, messageLine))
		if match == nil {
			return nil, errors.New(fmt.Sprintf("ReadMessages: bad channel/message response: %s,%s", channelOrOkline, messageLine))
		}
		msgs = append(msgs, ChannelMessage{match[1], match[2]})
	}
}

func (c *MPDClient) SendMessage(channel, text string) error {
	id, err := c.conn.Cmd(fmt.Sprintf(
		"sendmessage \"%s\" \"%s\"",
		channel,
		text,
	))
	if err != nil {
		return err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	line, err := c.conn.ReadLine()
	if line != "OK" {
		return errors.New("SendMessage: " + line)
	}

	return nil
}

func (c *MPDClient) Idle(subsystems ...string) (string, error) {
	var cmd string
	if len(subsystems) == 0 {
		cmd = "idle"
	} else {
		cmd = "idle " + strings.Join(subsystems, " ")
	}
	id, err := c.conn.Cmd(cmd)
	if err != nil {
		return "", err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	info := make(Info)
	err = fillInfoUntilOK(c, &info)
	if err != nil {
		return "", err
	}

	return info["changed"], nil
}

func (c *MPDClient) NoIdle() error {
	id, err := c.conn.Cmd("noidle")
	if err != nil {
		return err
	}
	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)

	return nil
}

func (c *MPDClient) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		if err != nil {
			return err
		}
		c.conn = nil
	}
	return nil
}

func ConnectDup(c *MPDClient) (*MPDClient, error) {
	return Connect(c.Host, c.Port)
}

func Connect(host string, port uint) (*MPDClient, error) {
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

	return &MPDClient{host, port, conn}, nil
}
