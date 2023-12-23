package log

import (
	"bufio"
	"bytes"

	"github.com/charmbracelet/log"
)

type Writer struct {
	Log *log.Logger
}

func (l *Writer) Write(p []byte) (n int, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line := scanner.Text()
		l.Log.Debug(line)
	}
	return len(p), nil
}
