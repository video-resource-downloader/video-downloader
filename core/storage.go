package core

import (
	"os"
	"path"

	"github.com/video-resource-downloader/video-downloader/core/shared"
)

type Storage struct {
	fileName string
	def      []byte
}

func NewStorage(app *App, filename string, def []byte) *Storage {
	return &Storage{
		fileName: path.Join(app.UserDir, filename),
		def:      def,
	}
}

func (l *Storage) Load() ([]byte, error) {
	if !shared.FileExist(l.fileName) {
		err := os.WriteFile(l.fileName, l.def, 0644)
		if err != nil {
			return nil, err
		}
		return l.def, nil
	}
	data, err := os.ReadFile(l.fileName)
	if err != nil {
		return nil, err
	}
	return data, err
}

func (l *Storage) Store(data []byte) error {
	return os.WriteFile(l.fileName, data, 0644)
}
