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
	"strconv"
	"strings"
)

const StickerSongType = "song"

type SongSticker struct {
	Uri   string
	Name  string
	Value string
}

type SongStickerList []SongSticker

func (p SongStickerList) Len() int { return len(p) }
func (p SongStickerList) Less(i, j int) bool {
	if p[i].Name != p[j].Name {
		return false
	}
	piVal, erri := strconv.Atoi(p[i].Value)
	pjVal, errj := strconv.Atoi(p[j].Value)
	if erri != nil || errj != nil {
		return false
	}
	return piVal < pjVal
}
func (p SongStickerList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
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

func (c *MPDClient) StickerFind(stype, uri, stickerName string) (SongStickerList, error) {
	res := c.Cmd(fmt.Sprintf(
		"sticker find \"%s\" \"%s\" \"%s\"",
		stype,
		uri,
		stickerName,
	))
	if res.Err != nil {
		return nil, res.Err
	}
	if res.MPDErr != nil {
		return nil, res.MPDErr
	}

	n := len(res.Data)
	songStickers := make(SongStickerList, n/2)
	for i := 0; i < n; i += 2 {
		matchFile := responseRegexp.FindStringSubmatch(res.Data[i])
		if matchFile[1] != "file" {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s, expected %s", matchFile[1], "file"))
		}
		matchSticker := responseRegexp.FindStringSubmatch(res.Data[i+1])
		if matchFile == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i]))
		}
		if matchSticker == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i+1]))
		}
		if matchSticker[1] != "sticker" {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s, expected %s", matchSticker[1], "sticker"))
		}
		pair := matchSticker[2]

		fieldSepIndex := strings.Index(pair, "=")
		if fieldSepIndex == -1 {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", pair))
		}
		var songSticker SongSticker
		songSticker.Uri = matchFile[2]
		songSticker.Name = pair[:fieldSepIndex]
		songSticker.Value = pair[fieldSepIndex+1:]
		songStickers[i/2] = songSticker
	}

	return songStickers, nil
}

func (c *MPDClient) Ping() error {
	res := c.Cmd("ping")
	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}
	return nil
}
