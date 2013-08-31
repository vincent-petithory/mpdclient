package mpdfav

import (
	"errors"
	"fmt"
	"strings"
)

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
