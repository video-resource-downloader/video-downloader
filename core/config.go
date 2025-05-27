package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
)

type MimeInfo struct {
	Type   string `json:"Type"`
	Suffix string `json:"Suffix"`
}

// Config struct
type Config struct {
	app           *App
	storage       *Storage
	Theme         string              `json:"Theme"`
	Locale        string              `json:"Locale"`
	Host          string              `json:"Host"`
	Port          string              `json:"Port"`
	Quality       int                 `json:"Quality"`
	SaveDirectory string              `json:"SaveDirectory"`
	FilenameLen   int                 `json:"FilenameLen"`
	FilenameTime  bool                `json:"FilenameTime"`
	UpstreamProxy string              `json:"UpstreamProxy"`
	OpenProxy     bool                `json:"OpenProxy"`
	DownloadProxy bool                `json:"DownloadProxy"`
	AutoProxy     bool                `json:"AutoProxy"`
	WxAction      bool                `json:"WxAction"`
	TaskNumber    int                 `json:"TaskNumber"`
	UserAgent     string              `json:"UserAgent"`
	UseHeaders    string              `json:"UseHeaders"`
	MimeMap       map[string]MimeInfo `json:"MimeMap"`
	mimeMux       sync.RWMutex
}

func newConfig(app *App) *Config {
	def := `
{
  "Host": "127.0.0.1",
  "Port": "22321",
  "Theme": "lightTheme",
  "Locale": "zh",
  "Quality": 0,
  "SaveDirectory": "",
  "FilenameLen": 0,
  "FilenameTime": true,
  "UpstreamProxy": "",
  "OpenProxy": false,
  "DownloadProxy": false,
  "AutoProxy": false,
  "WxAction": true,
  "TaskNumber": __TaskNumber__,
  "UserAgent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
  "UseHeaders": "User-Agent,Referer,Authorization,Cookie",
  "MimeMap": {
	  "image/png": { "Type": "image", "Suffix": ".png" },
	  "image/webp": { "Type": "image", "Suffix": ".webp" },
	  "image/jpeg": { "Type": "image", "Suffix": ".jpeg" },
	  "image/jpg": { "Type": "image", "Suffix": ".jpg" },
	  "image/gif": { "Type": "image", "Suffix": ".gif" },
	  "image/avif": { "Type": "image", "Suffix": ".avif" },
	  "image/bmp": { "Type": "image", "Suffix": ".bmp" },
	  "image/tiff": { "Type": "image", "Suffix": ".tiff" },
	  "image/heic": { "Type": "image", "Suffix": ".heic" },
	  "image/x-icon": { "Type": "image", "Suffix": ".ico" },
	  "image/svg+xml": { "Type": "image", "Suffix": ".svg" },
	  "image/vnd.adobe.photoshop": { "Type": "image", "Suffix": ".psd" },
	  "image/jp2": { "Type": "image", "Suffix": ".jp2" },
	  "image/jpeg2000": { "Type": "image", "Suffix": ".jp2" },
	  "image/apng": { "Type": "image", "Suffix": ".apng" },
	  "audio/mpeg": { "Type": "audio", "Suffix": ".mp3" },
	  "audio/mp3": { "Type": "audio", "Suffix": ".mp3" },
	  "audio/wav": { "Type": "audio", "Suffix": ".wav" },
	  "audio/aiff": { "Type": "audio", "Suffix": ".aiff" },
	  "audio/x-aiff": { "Type": "audio", "Suffix": ".aiff" },
	  "audio/aac": { "Type": "audio", "Suffix": ".aac" },
	  "audio/ogg": { "Type": "audio", "Suffix": ".ogg" },
	  "audio/flac": { "Type": "audio", "Suffix": ".flac" },
	  "audio/midi": { "Type": "audio", "Suffix": ".mid" },
	  "audio/x-midi": { "Type": "audio", "Suffix": ".mid" },
	  "audio/x-ms-wma": { "Type": "audio", "Suffix": ".wma" },
	  "audio/opus": { "Type": "audio", "Suffix": ".opus" },
	  "audio/webm": { "Type": "audio", "Suffix": ".webm" },
	  "audio/mp4": { "Type": "audio", "Suffix": ".m4a" },
	  "audio/amr": { "Type": "audio", "Suffix": ".amr" },
	  "video/mp4": { "Type": "video", "Suffix": ".mp4" },
	  "video/webm": { "Type": "video", "Suffix": ".webm" },
	  "video/ogg": { "Type": "video", "Suffix": ".ogv" },
	  "video/x-msvideo": { "Type": "video", "Suffix": ".avi" },
	  "video/mpeg": { "Type": "video", "Suffix": ".mpeg" },
	  "video/quicktime": { "Type": "video", "Suffix": ".mov" },
	  "video/x-ms-wmv": { "Type": "video", "Suffix": ".wmv" },
	  "video/3gpp": { "Type": "video", "Suffix": ".3gp" },
	  "video/x-matroska": { "Type": "video", "Suffix": ".mkv" },
	  "audio/video": { "Type": "live", "Suffix": ".flv" },
	  "video/x-flv": { "Type": "live", "Suffix": ".flv" },
	  "application/dash+xml": { "Type": "live", "Suffix": ".mpd" },
	  "application/vnd.apple.mpegurl": { "Type": "m3u8", "Suffix": ".m3u8" },
	  "application/x-mpegurl": { "Type": "m3u8", "Suffix": ".m3u8" },
	  "application/x-mpeg": { "Type": "m3u8", "Suffix": ".m3u8" },
	  "application/pdf": { "Type": "pdf", "Suffix": ".pdf" },
	  "application/vnd.ms-powerpoint": { "Type": "ppt", "Suffix": ".ppt" },
	  "application/vnd.openxmlformats-officedocument.presentationml.presentation": { "Type": "ppt", "Suffix": ".pptx" },
	  "application/vnd.ms-excel": { "Type": "xls", "Suffix": ".xls" },
	  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": { "Type": "xls", "Suffix": ".xlsx" },
	  "text/csv": { "Type": "xls", "Suffix": ".csv" },
	  "application/msword": { "Type": "doc", "Suffix": ".doc" },
	  "application/rtf": { "Type": "doc", "Suffix": ".rtf" },
	  "text/rtf": { "Type": "doc", "Suffix": ".rtf" },
	  "application/vnd.oasis.opendocument.text": { "Type": "doc", "Suffix": ".odt" },
	  "application/vnd.openxmlformats-officedocument.wordprocessingml.document": { "Type": "doc", "Suffix": ".docx" },
	  "font/woff": { "Type": "font", "Suffix": ".woff" }
	}
}
`
	def = strings.ReplaceAll(def, "__TaskNumber__", strconv.Itoa(runtime.NumCPU()*2))

	var cfg Config
	_ = json.Unmarshal([]byte(def), &cfg)
	storage := NewStorage(app, "config.json", []byte(def))
	if data, err := storage.Load(); err != nil {
		app.Logger.Esg(err, "load config err")
	} else {
		_ = json.Unmarshal(data, &cfg)
	}
	cfg.app = app
	cfg.storage = storage
	return &cfg
}

func (c *Config) setConfig(config *Config) {
	oldProxy := c.UpstreamProxy
	openProxy := c.OpenProxy
	c.Host = config.Host
	c.Port = config.Port
	c.Theme = config.Theme
	c.Locale = config.Locale
	c.Quality = config.Quality
	c.SaveDirectory = config.SaveDirectory
	if c.SaveDirectory == "" {
		dir, err := homedir.Dir()
		if err == nil {
			c.SaveDirectory = filepath.Join(dir, "Downloads", "video-downloader")
		}
		_ = os.MkdirAll(c.SaveDirectory, 0755)
	}
	c.FilenameLen = config.FilenameLen
	c.FilenameTime = config.FilenameTime
	c.UpstreamProxy = config.UpstreamProxy
	c.UserAgent = config.UserAgent
	c.OpenProxy = config.OpenProxy
	c.DownloadProxy = config.DownloadProxy
	c.AutoProxy = config.AutoProxy
	c.TaskNumber = config.TaskNumber
	c.WxAction = config.WxAction
	c.UseHeaders = config.UseHeaders
	if oldProxy != c.UpstreamProxy || openProxy != c.OpenProxy {
		c.app.httpServer.proxy.setTransport()
	}

	c.mimeMux.Lock()
	c.MimeMap = config.MimeMap
	c.mimeMux.Unlock()

	if data, err := json.Marshal(c); err == nil {
		_ = c.storage.Store(data)
	}
}

func (c *Config) getConfig(key string) interface{} {
	switch key {
	case "Host":
		return c.Host
	case "Port":
		return c.Port
	case "Theme":
		return c.Theme
	case "Locale":
		return c.Locale
	case "Quality":
		return c.Quality
	case "SaveDirectory":
		return c.SaveDirectory
	case "FilenameLen":
		return c.FilenameLen
	case "FilenameTime":
		return c.FilenameTime
	case "UpstreamProxy":
		return c.UpstreamProxy
	case "UserAgent":
		return c.UserAgent
	case "OpenProxy":
		return c.OpenProxy
	case "DownloadProxy":
		return c.DownloadProxy
	case "AutoProxy":
		return c.AutoProxy
	case "TaskNumber":
		return c.TaskNumber
	case "WxAction":
		return c.WxAction
	case "UseHeaders":
		return c.UseHeaders
	case "MimeMap":
		c.mimeMux.RLock()
		defer c.mimeMux.RUnlock()
		return c.MimeMap
	default:
		return nil
	}
}

func (c *Config) typeSuffix(mime string) (string, string) {
	c.mimeMux.RLock()
	defer c.mimeMux.RUnlock()
	mime = strings.ToLower(strings.Split(mime, ";")[0])
	if v, ok := c.MimeMap[mime]; ok {
		return v.Type, v.Suffix
	}
	return "", ""
}
