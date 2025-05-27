package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/video-resource-downloader/video-downloader/core/internal"
	"github.com/video-resource-downloader/video-downloader/core/shared"
)

type WxFileDecodeResult struct {
	SavePath string
	Message  string
}

type Resource struct {
	mediaMark  sync.Map
	resType    map[string]bool
	resTypeMux sync.RWMutex

	app        *App
	httpServer *HttpServer
}

func newResource(app *App, httpServer *HttpServer) *Resource {
	return &Resource{
		resType: map[string]bool{
			"all":   true,
			"image": true,
			"audio": true,
			"video": true,
			"m3u8":  true,
			"live":  true,
			"xls":   true,
			"doc":   true,
			"pdf":   true,
		},
		app:        app,
		httpServer: httpServer,
	}
}

func (r *Resource) mediaIsMarked(key string) bool {
	_, loaded := r.mediaMark.Load(key)
	return loaded
}

func (r *Resource) markMedia(key string) {
	r.mediaMark.Store(key, true)
}

func (r *Resource) getResType(key string) (bool, bool) {
	r.resTypeMux.RLock()
	defer r.resTypeMux.RUnlock()
	value, ok := r.resType[key]
	return value, ok
}

func (r *Resource) setResType(n []string) {
	r.resTypeMux.Lock()
	defer r.resTypeMux.Unlock()
	r.resType = map[string]bool{
		"all":   false,
		"image": false,
		"audio": false,
		"video": false,
		"m3u8":  false,
		"live":  false,
		"xls":   false,
		"doc":   false,
		"pdf":   false,
	}

	for _, value := range n {
		r.resType[value] = true
	}
}

func (r *Resource) clear() {
	r.mediaMark.Clear()
}

func (r *Resource) delete(sign string) {
	r.mediaMark.Delete(sign)
}

func (r *Resource) download(mediaInfo MediaInfo) {
	if r.app.cfg.SaveDirectory == "" {
		return
	}
	go func(mediaInfo MediaInfo) {
		rawUrl := mediaInfo.Url
		fileName := shared.Md5(rawUrl)
		if mediaInfo.Description != "" {
			fileName = regexp.MustCompile(`[^\w\p{Han}]`).ReplaceAllString(mediaInfo.Description, "")
			fileLen := r.app.cfg.FilenameLen
			if fileLen <= 0 {
				fileLen = 10
			}

			runes := []rune(fileName)
			if len(runes) > fileLen {
				fileName = string(runes[:fileLen])
			}
		}

		if r.app.cfg.FilenameTime {
			mediaInfo.SavePath = filepath.Join(r.app.cfg.SaveDirectory, fileName+"_"+shared.GetCurrentDateTimeFormatted()+mediaInfo.Suffix)
		} else {
			mediaInfo.SavePath = filepath.Join(r.app.cfg.SaveDirectory, fileName+mediaInfo.Suffix)
		}

		if strings.Contains(rawUrl, "qq.com") {
			if r.app.cfg.Quality == 1 &&
				strings.Contains(rawUrl, "encfilekey=") &&
				strings.Contains(rawUrl, "token=") {
				parseUrl, err := url.Parse(rawUrl)
				queryParams := parseUrl.Query()
				if err == nil && queryParams.Has("encfilekey") && queryParams.Has("token") {
					rawUrl = parseUrl.Scheme + "://" + parseUrl.Host + "/" + parseUrl.Path +
						"?encfilekey=" + queryParams.Get("encfilekey") +
						"&token=" + queryParams.Get("token")
				}
			} else if r.app.cfg.Quality > 1 && mediaInfo.OtherData["wx_file_formats"] != "" {
				format := strings.Split(mediaInfo.OtherData["wx_file_formats"], "#")
				qualityMap := []string{
					format[0],
					format[len(format)/2],
					format[len(format)-1],
				}
				rawUrl += "&X-snsvideoflag=" + qualityMap[r.app.cfg.Quality-2]
			}
		}

		headers, _ := r.parseHeaders(mediaInfo)

		downloader := NewFileDownloader(r.app, rawUrl, mediaInfo.SavePath, r.app.cfg.TaskNumber, headers)
		downloader.progressCallback = func(totalDownloaded, totalSize float64, taskID int, taskProgress float64) {
			r.progressEventsEmit(mediaInfo, strconv.Itoa(int(totalDownloaded*100/totalSize))+"%", shared.DownloadStatusRunning)
		}
		err := downloader.Start()
		if err != nil {
			r.progressEventsEmit(mediaInfo, err.Error())
			return
		}
		if mediaInfo.DecodeKey != "" {
			r.progressEventsEmit(mediaInfo, "decrypting in progress", shared.DownloadStatusRunning)
			decodedBytes := internal.GetDecryptorBytes(mediaInfo.DecodeKey)
			if err = r.decodeWxFile(mediaInfo.SavePath, decodedBytes); err != nil {
				r.progressEventsEmit(mediaInfo, "decryption error: "+err.Error())
				return
			}
		}
		r.progressEventsEmit(mediaInfo, "complete", shared.DownloadStatusDone)
	}(mediaInfo)
}

func (r *Resource) parseHeaders(mediaInfo MediaInfo) (map[string]string, error) {
	headers := make(map[string]string)

	if hh, ok := mediaInfo.OtherData["headers"]; ok {
		var tempHeaders map[string][]string
		if err := json.Unmarshal([]byte(hh), &tempHeaders); err != nil {
			return headers, fmt.Errorf("parse headers JSON err: %v", err)
		}

		for key, values := range tempHeaders {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	return headers, nil
}

func (r *Resource) wxFileDecode(mediaInfo MediaInfo, fileName string) (string, error) {
	sourceFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = sourceFile.Close()
	}()
	mediaInfo.SavePath = strings.ReplaceAll(fileName, ".mp4", "_decrypt.mp4")

	destinationFile, err := os.Create(mediaInfo.SavePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = destinationFile.Close()
	}()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return "", err
	}
	decodedBytes := internal.GetDecryptorBytes(mediaInfo.DecodeKey)
	err = r.decodeWxFile(mediaInfo.SavePath, decodedBytes)
	if err != nil {
		return "", err
	}
	return mediaInfo.SavePath, nil
}

func (r *Resource) progressEventsEmit(mediaInfo MediaInfo, args ...string) {
	Status := shared.DownloadStatusError
	Message := "ok"

	if len(args) > 0 {
		Message = args[0]
	}
	if len(args) > 1 {
		Status = args[1]
	}

	r.httpServer.send("downloadProgress", map[string]any{
		"Id":       mediaInfo.Id,
		"Status":   Status,
		"SavePath": mediaInfo.SavePath,
		"Message":  Message,
	})
	return
}

func (r *Resource) decodeWxFile(fileName string, decodedBytes []byte) error {
	file, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	byteCount := len(decodedBytes)
	fileBytes := make([]byte, byteCount)
	_, err = file.Read(fileBytes)
	if err != nil && err != io.EOF {
		return err
	}
	xorResult := make([]byte, byteCount)
	for i := 0; i < byteCount; i++ {
		xorResult[i] = decodedBytes[i] ^ fileBytes[i]
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = file.Write(xorResult)
	if err != nil {
		return err
	}
	return nil
}
