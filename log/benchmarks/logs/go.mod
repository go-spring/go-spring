module benchmark-logs

go 1.26

require (
	github.com/apex/log v1.9.0
	github.com/go-kit/log v0.2.1
	github.com/go-spring/log v0.0.12
	github.com/go-spring/stdlib v0.1.2
	github.com/rs/zerolog v1.34.0
	github.com/sirupsen/logrus v1.9.3
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	gopkg.in/inconshreveable/log15.v2 v2.16.0
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/term v0.12.0 // indirect
)

replace github.com/go-spring/log => ../../
