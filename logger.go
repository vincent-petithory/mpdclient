package mpdfav

import (
	"log"
	"os"
	"io"
	"fmt"
)

const logFilename = "mpdfav.log"

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