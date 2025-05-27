package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	sysRuntime "runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/video-resource-downloader/video-downloader/core/internal"
	"github.com/video-resource-downloader/video-downloader/core/shared"
)

type respData map[string]any

type ResponseData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type HttpServer struct {
	*gin.Engine
	app      *App
	resource *Resource
	proxy    *Proxy
}

func NewHttpServer(app *App) *HttpServer {
	httpServer := &HttpServer{
		Engine: gin.New(),
		app:    app,
	}
	httpServer.resource = newResource(app, httpServer)
	httpServer.proxy = newProxy(app, httpServer, httpServer.resource)
	httpServer.initRouter()
	return httpServer
}

func (h *HttpServer) initRouter() {
	// 注册CORS
	host := fmt.Sprintf("%s:%s", h.app.cfg.Host, h.app.cfg.Port)
	h.Use(func(c *gin.Context) {
		if c.Request.Host == host &&
			strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	})
	apiGroup := h.Group("api")
	apiGroup.POST("install", h.install)
	apiGroup.POST("set-system-password", h.setSystemPassword)
	apiGroup.GET("preview", h.preview)
	apiGroup.POST("proxy-open", h.openSystemProxy)
	apiGroup.POST("proxy-unset", h.unsetSystemProxy)
	apiGroup.POST("open-directory", h.openDirectoryDialog)
	apiGroup.POST("open-file", h.openFileDialog)
	apiGroup.POST("open-folder", h.openFolder)
	apiGroup.GET("is-proxy", h.isProxy)
	apiGroup.GET("app-info", h.appInfo)
	apiGroup.GET("get-config", h.getConfig)
	apiGroup.POST("set-config", h.setConfig)
	apiGroup.POST("set-type", h.setType)
	apiGroup.DELETE("clear", h.clear)
	apiGroup.POST("delete", h.delete)
	apiGroup.POST("download", h.download)
	apiGroup.POST("wx-file-decode", h.wxFileDecode)
	apiGroup.POST("wx-decode-keys", h.wxDecodeKeys)
	apiGroup.POST("batch-export", h.batchExport)
	apiGroup.GET("cert", h.downCert)
	// 路由未匹配走代理请求
	h.NoRoute(func(c *gin.Context) {
		h.proxy.Proxy.ServeHTTP(c.Writer, c.Request)
	})
}

func (h *HttpServer) run() {
	listener, err := net.Listen("tcp", h.app.cfg.Host+":"+h.app.cfg.Port)
	if err != nil {
		h.app.Logger.Err(err)
		log.Fatalf("Service cannot start: %v", err)
	}
	runtime.LogInfo(h.app.Context(), "Service started, listening http://"+h.app.cfg.Host+":"+h.app.cfg.Port)
	if err1 := http.Serve(listener, h); err1 != nil {
		h.app.Logger.Err(err1)
		runtime.LogInfof(h.app.Context(), "Service startup exception: %v", err1)
	}
}

func (h *HttpServer) downCert(c *gin.Context) {
	c.Header("Content-Type", "application/x-x509-ca-data")
	c.Header("Content-Disposition", "attachment;filename=video-downloader-public.crt")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Length", fmt.Sprintf("%d", len(h.app.PublicCrt)))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, io.NopCloser(bytes.NewReader(h.app.PublicCrt)))
}

func (h *HttpServer) preview(c *gin.Context) {
	savePath := c.Query("savePath")
	if savePath != "" {
		// 检查文件是否存在
		if _, err := os.Stat(savePath); os.IsNotExist(err) {
			c.String(http.StatusNotFound, "File not found")
			return
		}
		// 获取文件名
		filename := filepath.Base(savePath)
		// 设置响应头
		c.Header("Content-Type", "video/mp4") // 可根据实际类型调整
		c.Header("Content-Disposition", "inline; filename="+filename)
		c.Header("Content-Transfer-Encoding", "binary")
		c.File(savePath)
		return
	}
	realURL := c.Query("url")
	if realURL == "" {
		c.String(http.StatusBadRequest, "Missing 'url' parameter")
		return
	}
	realURL, _ = url.QueryUnescape(realURL)
	parsedURL, err := url.Parse(realURL)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid URL")
		return
	}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create request")
		return
	}

	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}

	//request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")
	//request.Header.Set("Referer", parsedURL.Scheme+"://"+parsedURL.Host+"/")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch the resource")
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Status(resp.StatusCode)

	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		c.Header("Content-Range", contentRange)
	}

	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to serve the resource")
	}
	return
}

func (h *HttpServer) send(t string, data any) {
	jsonData, err := json.Marshal(map[string]any{
		"type": t,
		"data": data,
	})
	if err != nil {
		runtime.LogErrorf(h.app.Context(), "Error converting map to JSON: %v", err)
		return
	}
	runtime.EventsEmit(h.app.Context(), "event", string(jsonData))
}

func (h *HttpServer) writeJson(w http.ResponseWriter, data *ResponseData) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		h.app.Logger.Err(err)
	}
}

func (h *HttpServer) error(c *gin.Context, args ...any) {
	message := "ok"
	var data any

	if len(args) > 0 {
		message = args[0].(string)
	}
	if len(args) > 1 {
		data = args[1]
	}
	c.JSON(http.StatusOK, buildResp(0, message, data))
}

func (h *HttpServer) success(c *gin.Context, args ...any) {
	message := "ok"
	var data any

	if len(args) > 0 {
		data = args[0]
	}

	if len(args) > 1 {
		message = args[1].(string)
	}
	c.JSON(http.StatusOK, buildResp(1, message, data))
}

func (h *HttpServer) openDirectoryDialog(c *gin.Context) {
	folder, err := runtime.OpenDirectoryDialog(h.app.Context(), runtime.OpenDialogOptions{
		DefaultDirectory: "",
		Title:            "Select a folder",
	})
	if err != nil {
		h.error(c, err.Error())
		return
	}
	h.success(c, respData{
		"folder": folder,
	})
}

func (h *HttpServer) openFileDialog(c *gin.Context) {
	filePath, err := runtime.OpenFileDialog(h.app.Context(), runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Videos (*.mov;*.mp4)",
				Pattern:     "*.mp4",
			},
		},
		Title: "Select a file",
	})
	if err != nil {
		h.error(c, err.Error())
		return
	}
	h.success(c, respData{
		"file": filePath,
	})
}

func (h *HttpServer) openFolder(c *gin.Context) {
	var data struct {
		FilePath string `json:"filePath"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c, err.Error())
		return
	}
	filePath := data.FilePath
	var cmd *exec.Cmd
	switch sysRuntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", filePath)
	case "windows":
		cmd = exec.Command("explorer", "/select,", filePath)
	case "linux":
		cmd = exec.Command("nautilus", filePath)
		if err := cmd.Start(); err != nil {
			cmd = exec.Command("thunar", filePath)
			if err := cmd.Start(); err != nil {
				cmd = exec.Command("dolphin", filePath)
				if err := cmd.Start(); err != nil {
					cmd = exec.Command("pcmanfm", filePath)
					if err := cmd.Start(); err != nil {
						h.app.Logger.Err(err)
						h.error(c, err.Error())
						return
					}
				}
			}
		}
	default:
		h.error(c, "unsupported platform")
		return
	}

	if err := cmd.Start(); err != nil {
		h.app.Logger.Err(err)
		h.error(c, err.Error())
		return
	}
	h.success(c)
}

func (h *HttpServer) install(c *gin.Context) {
	if h.app.isInstall() {
		h.success(c, respData{
			"isPass": h.app.system.Password == "",
		})
		return
	}

	out, err := h.app.installCert()
	if err != nil {
		h.error(c, err.Error()+"\n"+out, respData{
			"isPass": h.app.system.Password == "",
		})
		return
	}

	h.success(c, respData{
		"isPass": h.app.system.Password == "",
	})
}

func (h *HttpServer) setSystemPassword(c *gin.Context) {
	var data struct {
		Password string `json:"password"`
		IsCache  bool   `json:"isCache"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c, err.Error())
		return
	}
	h.app.system.SetPassword(data.Password, data.IsCache)
	h.success(c)
}

func (h *HttpServer) openSystemProxy(c *gin.Context) {
	err := h.app.OpenSystemProxy()
	if err != nil {
		h.error(c, err.Error(), respData{
			"value": h.app.IsProxy,
		})
		return
	}
	h.success(c, respData{
		"value": h.app.IsProxy,
	})
}

func (h *HttpServer) unsetSystemProxy(c *gin.Context) {
	err := h.app.UnsetSystemProxy()
	if err != nil {
		h.error(c, err.Error(), respData{
			"value": h.app.IsProxy,
		})
		return
	}
	h.success(c, respData{
		"value": h.app.IsProxy,
	})
}

func (h *HttpServer) isProxy(c *gin.Context) {
	h.success(c, respData{
		"value": h.app.IsProxy,
	})
}

func (h *HttpServer) appInfo(c *gin.Context) {
	h.success(c, h.app)
}

func (h *HttpServer) getConfig(c *gin.Context) {
	h.success(c, h.app.cfg)
}

func (h *HttpServer) setConfig(c *gin.Context) {
	var data Config
	if err := c.BindJSON(&data); err != nil {
		h.error(c, err.Error())
		return
	}
	h.app.cfg.setConfig(&data)
	h.success(c)
}

func (h *HttpServer) setType(c *gin.Context) {
	var data struct {
		Type string `json:"type"`
	}
	err := c.BindJSON(&data)
	if err == nil {
		if data.Type != "" {
			h.resource.setResType(strings.Split(data.Type, ","))
		} else {
			h.resource.setResType([]string{})
		}
	}

	h.success(c)
}

func (h *HttpServer) clear(c *gin.Context) {
	h.resource.clear()
	h.success(c)
}

func (h *HttpServer) delete(c *gin.Context) {
	var data struct {
		Sign string `json:"sign"`
	}
	err := c.BindJSON(&data)
	if err == nil && data.Sign != "" {
		h.resource.delete(data.Sign)
	}
	h.success(c)
}

func (h *HttpServer) download(c *gin.Context) {
	var data struct {
		MediaInfo
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c, err.Error())
		return
	}
	h.resource.download(data.MediaInfo)
	h.success(c)
}

func (h *HttpServer) wxFileDecode(c *gin.Context) {
	var data struct {
		MediaInfo
		Filename string `json:"filename"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c, err.Error())
		return
	}
	savePath, err := h.resource.wxFileDecode(data.MediaInfo, data.Filename)
	if err != nil {
		h.error(c, err.Error())
		return
	}
	h.success(c, respData{
		"save_path": savePath,
	})
}

func (h *HttpServer) wxDecodeKeys(c *gin.Context) {
	var mediaInfo MediaInfo
	if err := c.BindJSON(&mediaInfo); err != nil {
		h.error(c, err.Error())
		return
	}
	decryptorArray := internal.GetDecryptorBytes(mediaInfo.DecodeKey)
	h.success(c, respData{
		"decryptorBase64": base64.StdEncoding.EncodeToString(decryptorArray),
	})
}

func (h *HttpServer) batchExport(c *gin.Context) {
	var data struct {
		Content string `json:"content"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c, err.Error())
		return
	}
	fileName := filepath.Join(h.app.cfg.SaveDirectory, "video-downloader-"+shared.GetCurrentDateTimeFormatted()+".txt")
	err := os.WriteFile(fileName, []byte(data.Content), 0644)
	if err != nil {
		h.error(c, err.Error())
		return
	}
	h.success(c, respData{
		"file_name": fileName,
	})
}
