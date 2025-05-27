package core

type Bind struct {
	app *App
}

func NewBind(app *App) *Bind {
	return &Bind{
		app: app,
	}
}

func (b *Bind) Config() *ResponseData {
	return buildResp(1, "ok", b.app.cfg)
}

func (b *Bind) AppInfo() *ResponseData {
	return buildResp(1, "ok", b.app)
}
