package shared

import (
	"net/http"

	"github.com/elazarl/goproxy"
)

type Bridge struct {
	GetVersion    func() string
	GetResType    func(key string) (bool, bool)
	TypeSuffix    func(mime string) (string, string)
	MediaIsMarked func(key string) bool
	MarkMedia     func(key string)
	GetConfig     func(key string) any
	Send          func(t string, data any)
}

type Plugin interface {
	SetBridge(*Bridge)
	Domains() []string
	OnRequest(*http.Request, *goproxy.ProxyCtx) (*http.Request, *http.Response)
	OnResponse(*http.Response, *goproxy.ProxyCtx) *http.Response
}
