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
	"time"
)

type PlaylistInfo struct {
	Name         string
	LastModified *time.Time
}

const PlaylistInfoLastModifiedTimeLayout = "2006-01-02T15:04:05Z"

func (c *MPDClient) ListPlaylists() ([]PlaylistInfo, error) {
	res := c.Cmd("listplaylists")
	if res.Err != nil {
		return nil, res.Err
	}
	if res.MPDErr != nil {
		return nil, res.MPDErr
	}

	n := len(res.Data)
	playlistsInfo := make([]PlaylistInfo, n/2)
	for i := 0; i < n; i += 2 {
		matchName := responseRegexp.FindStringSubmatch(res.Data[i])
		matchLastModified := responseRegexp.FindStringSubmatch(res.Data[i+1])
		if matchName == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i]))
		}
		if matchLastModified == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", res.Data[i+1]))
		}
		var playlistInfo *PlaylistInfo = &playlistsInfo[i/2]
		playlistInfo.Name = matchName[2]
		lastModified, err := time.Parse(PlaylistInfoLastModifiedTimeLayout, matchLastModified[2])
		if err == nil {
			playlistInfo.LastModified = &lastModified
		}
	}

	return playlistsInfo, nil
}

func (c *MPDClient) Save(name string) error {
	res := c.Cmd(fmt.Sprintf(
		"save \"%s\"",
		name,
	))
	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}

	return nil
}

func (c *MPDClient) Rm(name string) error {
	res := c.Cmd(fmt.Sprintf(
		"rm \"%s\"",
		name,
	))
	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}

	return nil
}

func (c *MPDClient) PlaylistClear(name string) error {
	res := c.Cmd(fmt.Sprintf(
		"playlistclear \"%s\"",
		name,
	))
	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}

	return nil
}

func (c *MPDClient) ListPlaylist(name string) ([]string, error) {
	res := c.Cmd(fmt.Sprintf(
		"listplaylist \"%s\"",
		name,
	))
	if res.Err != nil {
		return nil, res.Err
	}
	if res.MPDErr != nil {
		return nil, res.MPDErr
	}

	songs := make([]string, len(res.Data))
	for i, songEntry := range res.Data {
		matchSong := responseRegexp.FindStringSubmatch(songEntry)
		if matchSong == nil {
			return nil, errors.New(fmt.Sprintf("Invalid input: %s", songEntry))
		}
		songs[i] = matchSong[2]
	}

	return songs, nil
}

func (c *MPDClient) PlaylistAdd(name, uri string) error {
	res := c.Cmd(fmt.Sprintf(
		"playlistadd \"%s\" \"%s\"",
		name,
		uri,
	))
	if res.Err != nil {
		return res.Err
	}
	if res.MPDErr != nil {
		return res.MPDErr
	}

	return nil
}
