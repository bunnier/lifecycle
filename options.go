package lifecycle

import (
	"context"
	"log"
	"net/http"
)

// WithHttpServer 用于构建向App注册Server的选项函数。
func WithHttpServer(server *http.Server) AppOption {
	return func(app *App) {
		app.httpServers = append(app.httpServers, server)
	}
}

// WithLog 用于构建向App注册Server的选项函数。
func WithLog(logger *log.Logger) AppOption {
	return func(app *App) {
		app.logger = logger
	}
}

// WithLog 用于构建向App注册Server的选项函数。
func WithContext(ctx context.Context) AppOption {
	return func(app *App) {
		app.ctx = ctx
	}
}
