package mpdfav

import (
	"fmt"
	"errors"
	"strconv"
	"strings"
	"net/textproto"
	"regexp"
)

const (
	network = "tcp"
	infoFieldSep = ": "
)

type Info map[string]string

var stickerGetRegexp = regexp.MustCompile("sticker: (.*)=(.*)")

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
		if strings.Index(line, "ACK") == -1 {
			return "", nil
		}
		return "", errors.New("StickerGet: "+line)
	}
	value := match[2]
	// OK line appears if it's ok,
	// otherwise only one line with error
	okLine, err := c.conn.ReadLine()
	if err != nil {
		return "", err
	}
	if okLine != "OK" {
		return "", errors.New("StickerGet didn't receive OK line: "+okLine)
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
		return errors.New("StickerSet: "+line)
	}

	return nil
}

func (c *MPDClient) Idle(subsystem string) (Info, error) {
	var cmd string
	if subsystem == "" {
		cmd = "idle"
	} else {
		cmd = "idle " + subsystem
	}
	id, err := c.conn.Cmd(cmd)
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
	mpdc := MPDClient{}
	mpdc.Host = host
	mpdc.Port = port
	mpdc.conn = conn
	return &mpdc, nil
}