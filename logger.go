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
	"fmt"
	"io"
	"log"
	"os"
)

const logFilename = "mpdclient.log"

func newMPDCLogger(uid uint, stderr bool) (*log.Logger, error) {
	var filename string
	if stderr {
		filename = logFilename
	} else {
		filename = os.ExpandEnv(fmt.Sprintf("$HOME/.%s", logFilename))
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	var writer io.Writer
	if stderr {
		writer = io.MultiWriter(os.Stderr, file)
	} else {
		writer = file
	}
	prefix := fmt.Sprintf("MPDClient(%d) > ", uid)
	return log.New(writer, prefix, log.LstdFlags|log.Lmicroseconds|log.Lshortfile), nil

}
