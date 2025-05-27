package core

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/elazarl/goproxy"

	"github.com/video-resource-downloader/video-downloader/core/plugins"
	"github.com/video-resource-downloader/video-downloader/core/shared"
)

type Proxy struct {
	app            *App
	Proxy          *goproxy.ProxyHttpServer
	Is             bool
	pluginRegistry map[string]shared.Plugin
}

type MediaInfo struct {
	Id          string
	Url         string
	UrlSign     string
	CoverUrl    string
	Size        string
	Domain      string
	Classify    string
	Suffix      string
	SavePath    string
	Status      string
	DecodeKey   string
	Description string
	ContentType string
	OtherData   map[string]string
}

func newProxy(app *App, httpServer *HttpServer, resource *Resource) *Proxy {
	proxy := &Proxy{
		app:            app,
		pluginRegistry: make(map[string]shared.Plugin),
	}
	ps := []shared.Plugin{
		&plugins.QqPlugin{},
		&plugins.DefaultPlugin{},
	}

	bridge := &shared.Bridge{
		GetVersion: func() string {
			return app.Version
		},
		GetResType: func(key string) (bool, bool) {
			return resource.getResType(key)
		},
		TypeSuffix: func(mine string) (string, string) {
			return app.cfg.typeSuffix(mine)
		},
		MediaIsMarked: func(key string) bool {
			return resource.mediaIsMarked(key)
		},
		MarkMedia: func(key string) {
			resource.markMedia(key)
		},
		GetConfig: func(key string) any {
			return app.cfg.getConfig(key)
		},
		Send: func(t string, data any) {
			httpServer.send(t, data)
		},
	}

	for _, p := range ps {
		p.SetBridge(bridge)
		for _, domain := range p.Domains() {
			proxy.pluginRegistry[domain] = p
		}
	}

	proxy.Startup()
	return proxy
}

func (p *Proxy) Startup() {
	err := p.setCa()
	if err != nil {
		DialogErr(p.app.Context(), "Failed to start proxy service："+err.Error())
		return
	}

	p.Proxy = goproxy.NewProxyHttpServer()
	//p.Proxy.KeepDestinationHeaders = true
	//p.Proxy.Verbose = false
	p.setTransport()
	p.Proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	p.Proxy.OnRequest().DoFunc(p.httpRequestEvent)
	p.Proxy.OnResponse().DoFunc(p.httpResponseEvent)
}

func (p *Proxy) setCa() error {
	ca, err := tls.X509KeyPair(p.app.PublicCrt, p.app.PrivateKey)
	if err != nil {
		DialogErr(p.app.Context(), "Failed to start proxy service 1")
		return err
	}
	if ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = ca
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	return nil
}

func (p *Proxy) setTransport() {
	transport := &http.Transport{
		DisableKeepAlives: false,
		// MaxIdleConnsPerHost: 10,
		DialContext: (&net.Dialer{
			Timeout: 60 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   60 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}

	if p.app.cfg.UpstreamProxy != "" && p.app.cfg.OpenProxy && !strings.Contains(p.app.cfg.UpstreamProxy, p.app.cfg.Port) {
		proxyURL, err := url.Parse(p.app.cfg.UpstreamProxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	p.Proxy.Tr = transport
}

func (p *Proxy) matchPlugin(host string) shared.Plugin {
	domain := shared.GetTopLevelDomain(host)
	if plugin, ok := p.pluginRegistry[domain]; ok {
		return plugin
	}
	return nil
}

func (p *Proxy) httpRequestEvent(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	plugin := p.matchPlugin(r.Host)
	if plugin != nil {
		newReq, newResp := plugin.OnRequest(r, ctx)
		if newResp != nil {
			return newReq, newResp
		}

		if newReq != nil {
			return newReq, nil
		}
	}
	return p.pluginRegistry["default"].OnRequest(r, ctx)
}

func (p *Proxy) httpResponseEvent(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.Request == nil {
		return resp
	}

	plugin := p.matchPlugin(resp.Request.Host)
	if plugin != nil {
		newResp := plugin.OnResponse(resp, ctx)
		if newResp != nil {
			return newResp
		}
	}

	return p.pluginRegistry["default"].OnResponse(resp, ctx)
}
