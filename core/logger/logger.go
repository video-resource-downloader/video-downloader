package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/video-resource-downloader/video-downloader/core/shared"
)

type Logger struct {
	zerolog.Logger
	logFile *os.File
}

// NewLogger create a new logger
func NewLogger(logFile bool, logPath string) *Logger {
	var out io.Writer
	if logFile {
		// log to file
		logDir := filepath.Dir(logPath)
		if err := shared.CreateDirIfNotExist(logDir); err != nil {
			panic(err)
		}
		var (
			logfile *os.File
			err     error
		)
		logfile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			panic(err)
		}
		out = logfile
	} else {
		out = os.Stdout
	}

	logger := &Logger{}
	if logFile {
		logger.logFile = out.(*os.File)
	}
	logger.Logger = zerolog.New(zerolog.ConsoleWriter{
		NoColor:    true,
		Out:        out,
		TimeFormat: "2006-01-02 15:04:05",
	}).With().Timestamp().Logger()
	return logger
}

func (l *Logger) Close() {
	_ = l.logFile.Close()
}

func (l *Logger) Err(err error) {
	l.Error().Stack().Err(err)
}

func (l *Logger) Esg(err error, format string, v ...any) {
	l.Error().Stack().Err(err).Msgf(fmt.Sprintf(format, v...))
}
