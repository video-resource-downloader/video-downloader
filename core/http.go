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

type respData map[string]interface{}

type ResponseData struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type HttpServer struct {
	*gin.Engine
}

func initHttpServer() *HttpServer {
	if httpServerOnce == nil {
		httpServerOnce = &HttpServer{
			Engine: gin.New(),
		}
		httpServerOnce.initRouter()
	}
	return httpServerOnce
}

func (h *HttpServer) initRouter() {
	// 注册CORS
	host := fmt.Sprintf("%s:%s", globalConfig.Host, globalConfig.Port)
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
		proxyOnce.Proxy.ServeHTTP(c.Writer, c.Request)
	})
}

func (h *HttpServer) run() {
	listener, err := net.Listen("tcp", globalConfig.Host+":"+globalConfig.Port)
	if err != nil {
		globalLogger.Err(err)
		log.Fatalf("Service cannot start: %v", err)
	}
	fmt.Println("Service started, listening http://" + globalConfig.Host + ":" + globalConfig.Port)
	if err1 := http.Serve(listener, h); err1 != nil {
		globalLogger.Err(err1)
		fmt.Printf("Service startup exception: %v", err1)
	}
}

func (h *HttpServer) downCert(c *gin.Context) {
	c.Header("Content-Type", "application/x-x509-ca-data")
	c.Header("Content-Disposition", "attachment;filename=video-downloader-public.crt")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Length", fmt.Sprintf("%d", len(appOnce.PublicCrt)))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, io.NopCloser(bytes.NewReader(appOnce.PublicCrt)))
}

func (h *HttpServer) preview(c *gin.Context) {
	savePath := c.Query("savePath")
	if savePath != "" {
		// 检查文件是否存在
		if _, err := os.Stat(savePath); os.IsNotExist(err) {
			http.Error(c.Writer, "File not found", http.StatusNotFound)
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
		http.Error(c.Writer, "Missing 'url' parameter", http.StatusBadRequest)
		return
	}
	realURL, _ = url.QueryUnescape(realURL)
	parsedURL, err := url.Parse(realURL)
	if err != nil {
		http.Error(c.Writer, "Invalid URL", http.StatusBadRequest)
		return
	}
	request, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		http.Error(c.Writer, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}

	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}

	//request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")
	//request.Header.Set("Referer", parsedURL.Scheme+"://"+parsedURL.Host+"/")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		http.Error(c.Writer, "Failed to fetch the resource", http.StatusInternalServerError)
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
		http.Error(c.Writer, "Failed to serve the resource", http.StatusInternalServerError)
	}
	return
}

func (h *HttpServer) send(t string, data interface{}) {
	jsonData, err := json.Marshal(map[string]interface{}{
		"type": t,
		"data": data,
	})
	if err != nil {
		fmt.Println("Error converting map to JSON:", err)
		return
	}
	runtime.EventsEmit(appOnce.ctx, "event", string(jsonData))
}

func (h *HttpServer) writeJson(w http.ResponseWriter, data *ResponseData) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		globalLogger.Err(err)
	}
}

func (h *HttpServer) error(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		message = args[0].(string)
	}
	if len(args) > 1 {
		data = args[1]
	}
	h.writeJson(w, h.buildResp(0, message, data))
}

func (h *HttpServer) success(w http.ResponseWriter, args ...interface{}) {
	message := "ok"
	var data interface{}

	if len(args) > 0 {
		data = args[0]
	}

	if len(args) > 1 {
		message = args[1].(string)
	}
	h.writeJson(w, h.buildResp(1, message, data))
}

func (h *HttpServer) buildResp(code int, message string, data interface{}) *ResponseData {
	return &ResponseData{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func (h *HttpServer) openDirectoryDialog(c *gin.Context) {
	folder, err := runtime.OpenDirectoryDialog(appOnce.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: "",
		Title:            "Select a folder",
	})
	if err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	h.success(c.Writer, respData{
		"folder": folder,
	})
}

func (h *HttpServer) openFileDialog(c *gin.Context) {
	filePath, err := runtime.OpenFileDialog(appOnce.ctx, runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Videos (*.mov;*.mp4)",
				Pattern:     "*.mp4",
			},
		},
		Title: "Select a file",
	})
	if err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	h.success(c.Writer, respData{
		"file": filePath,
	})
}

func (h *HttpServer) openFolder(c *gin.Context) {
	var data struct {
		FilePath string `json:"filePath"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c.Writer, err.Error())
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
						globalLogger.Err(err)
						h.error(c.Writer, err.Error())
						return
					}
				}
			}
		}
	default:
		h.error(c.Writer, "unsupported platform")
		return
	}

	if err := cmd.Start(); err != nil {
		globalLogger.Err(err)
		h.error(c.Writer, err.Error())
		return
	}
	h.success(c.Writer)
}

func (h *HttpServer) install(c *gin.Context) {
	if appOnce.isInstall() {
		h.success(c.Writer, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	out, err := appOnce.installCert()
	if err != nil {
		h.error(c.Writer, err.Error()+"\n"+out, respData{
			"isPass": systemOnce.Password == "",
		})
		return
	}

	h.success(c.Writer, respData{
		"isPass": systemOnce.Password == "",
	})
}

func (h *HttpServer) setSystemPassword(c *gin.Context) {
	var data struct {
		Password string `json:"password"`
		IsCache  bool   `json:"isCache"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	systemOnce.SetPassword(data.Password, data.IsCache)
	h.success(c.Writer)
}

func (h *HttpServer) openSystemProxy(c *gin.Context) {
	err := appOnce.OpenSystemProxy()
	if err != nil {
		h.error(c.Writer, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(c.Writer, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) unsetSystemProxy(c *gin.Context) {
	err := appOnce.UnsetSystemProxy()
	if err != nil {
		h.error(c.Writer, err.Error(), respData{
			"value": appOnce.IsProxy,
		})
		return
	}
	h.success(c.Writer, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) isProxy(c *gin.Context) {
	h.success(c.Writer, respData{
		"value": appOnce.IsProxy,
	})
}

func (h *HttpServer) appInfo(c *gin.Context) {
	h.success(c.Writer, appOnce)
}

func (h *HttpServer) getConfig(c *gin.Context) {
	h.success(c.Writer, globalConfig)
}

func (h *HttpServer) setConfig(c *gin.Context) {
	var data Config
	if err := c.BindJSON(&data); err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	globalConfig.setConfig(data)
	h.success(c.Writer)
}

func (h *HttpServer) setType(c *gin.Context) {
	var data struct {
		Type string `json:"type"`
	}
	err := c.BindJSON(&data)
	if err == nil {
		if data.Type != "" {
			resourceOnce.setResType(strings.Split(data.Type, ","))
		} else {
			resourceOnce.setResType([]string{})
		}
	}

	h.success(c.Writer)
}

func (h *HttpServer) clear(c *gin.Context) {
	resourceOnce.clear()
	h.success(c.Writer)
}

func (h *HttpServer) delete(c *gin.Context) {
	var data struct {
		Sign string `json:"sign"`
	}
	err := c.BindJSON(&data)
	if err == nil && data.Sign != "" {
		resourceOnce.delete(data.Sign)
	}
	h.success(c.Writer)
}

func (h *HttpServer) download(c *gin.Context) {
	var data struct {
		MediaInfo
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	resourceOnce.download(data.MediaInfo)
	h.success(c.Writer)
}

func (h *HttpServer) wxFileDecode(c *gin.Context) {
	var data struct {
		MediaInfo
		Filename string `json:"filename"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	savePath, err := resourceOnce.wxFileDecode(data.MediaInfo, data.Filename)
	if err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	h.success(c.Writer, respData{
		"save_path": savePath,
	})
}

func (h *HttpServer) wxDecodeKeys(c *gin.Context) {
	var mediaInfo MediaInfo
	if err := c.BindJSON(&mediaInfo); err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	decryptorArray := internal.GetDecryptorBytes(mediaInfo.DecodeKey)
	h.success(c.Writer, respData{
		"decryptorBase64": base64.StdEncoding.EncodeToString(decryptorArray),
	})
}

func (h *HttpServer) batchExport(c *gin.Context) {
	var data struct {
		Content string `json:"content"`
	}
	if err := c.BindJSON(&data); err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	fileName := filepath.Join(globalConfig.SaveDirectory, "video-downloader-"+shared.GetCurrentDateTimeFormatted()+".txt")
	err := os.WriteFile(fileName, []byte(data.Content), 0644)
	if err != nil {
		h.error(c.Writer, err.Error())
		return
	}
	h.success(c.Writer, respData{
		"file_name": fileName,
	})
}
