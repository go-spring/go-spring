package app

type builder struct {
	app *application
}

func New() *builder {
	return &builder{app: NewApplication()}
}

func (bu *builder) BannerMode(mode BannerMode) *builder {
	bu.app.SetBannerMode(mode)
	return bu
}

func (bu *builder) ExpectSysProperties(pattern ...string) *builder {
	bu.app.ExpectSysProperties(pattern...)
	return bu
}

func (bu *builder) AfterPrepare(fn AfterPrepareFunc) *builder {
	bu.app.AfterPrepare(fn)
	return bu
}

func (bu *builder) Run(cfgLocation ...string) {
	bu.app.Run(cfgLocation...)
}
