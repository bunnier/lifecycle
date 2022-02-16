package lifecycle

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// App 是一个Server生命周期托管对象。
type App struct {
	ctx             context.Context  // app 的核心上下文。
	cancel          func()           // 用于协调 goroutine 的退出。
	logger          *log.Logger      // 内部日志。
	httpServers     []*http.Server   // 维护的 http 服务。
	grpcServerInfos []GrpcServerInfo // 维护的 grpc 服务。
	errGroup        *errgroup.Group  // 用于协调 goroutine。
}

// App 的选项函数。
type AppOption func(app *App)

// NewApp 返回一个 App 对象。
func NewApp(opts ...AppOption) *App {
	app := &App{
		ctx:             context.Background(),
		logger:          log.Default(),
		httpServers:     make([]*http.Server, 0, 3),
		grpcServerInfos: make([]GrpcServerInfo, 0, 3),
	}

	for _, opt := range opts {
		opt(app)
	}

	app.ctx, app.cancel = context.WithCancel(app.ctx)

	// 注意，后面直接用 errGroup 的 ctx 来做上下文控制，由于 errGroup 的上下文是个子上下文，app 上挂着的 cancel 也能取消它。
	app.errGroup, app.ctx = errgroup.WithContext(app.ctx)

	return app
}

// Start 用来开启服务。
func (a *App) Run() error {
	a.startListenSystemSignal() // 开启系统信号监听。
	a.startHttpServers()        // 开启托管的 Http 服务。
	a.startGrpcServers()        // 开启托管的 Grpc 服务。
	return a.errGroup.Wait()    // 等待 errGroup 的结束信号。
}

// Stop 用来关闭服务。
func (a *App) Stop() error {
	a.logger.Println("start to stop app...")
	a.cancel() // ctx 的 cancel 联动了 errGroup 起的 goroutine 中的 shutdown 等，所以后面的 Wait 可以很快返回。
	return a.errGroup.Wait()
}

// startHttpServers 用来开启托管的服务。
func (a *App) startHttpServers() {
	for _, srv := range a.httpServers {
		srv := srv
		a.errGroup.Go(func() error {
			// 每一个服务起一个 goroutine 来监听 shutdown 信号。
			go func() {
				<-a.ctx.Done() // 上下文对象被取消后，各个服务就都自行了结了吧～
				a.logger.Printf("start to shutdown http server: %v\n", srv.Addr)
				srv.Shutdown(context.TODO())
			}()

			a.logger.Printf("start http server: %v\n", srv.Addr)
			// 正式开启服务，阻塞方法，shutdown 后这个方法才会返回。
			return errors.WithMessagef(srv.ListenAndServe(), "http server exit: %s", srv.Addr)
		})
	}
}

// startHttpServers 用来开启托管的服务。
func (a *App) startGrpcServers() {
	for _, srv := range a.grpcServerInfos {
		srv := srv
		a.errGroup.Go(func() error {
			// 每一个服务起一个 goroutine 来监听 shutdown 信号。
			go func() {
				<-a.ctx.Done() // 上下文对象被取消后，各个服务就都自行了结了吧～
				a.logger.Printf("start to shutdown grpc server: %v\n", srv.EndPoint)
				srv.GrpcServer.GracefulStop()
			}()

			a.logger.Printf("start grpc server: %v\n", srv.EndPoint)
			tcp, err := net.Listen("tcp", srv.EndPoint)
			if err != nil {
				return errors.Wrap(err, "grpc startup: tcp")
			}
			// 正式开启服务，阻塞方法，GracefulStop 后这个方法才会返回。
			return errors.WithMessagef(srv.GrpcServer.Serve(tcp), "grpc server exit: %s", srv.EndPoint)
		})
	}
}

// startListenSystemSignal 开启系统信号监听。
func (a *App) startListenSystemSignal() {
	// 在 errGroup 中起一个用于监听系统信号的 goroutine。
	a.errGroup.Go(func() error {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM) // 把系统信号注册到 signalCh 中。

		select {
		case <-a.ctx.Done():
			signal.Stop(signalCh) // 停止系统信号的监听。
			close(signalCh)
			return a.ctx.Err()
		case signal := <-signalCh:
			// 这边返回 error 给 errGroup 后，errGroup 会调用 context 的 cancel，令其它的 goroutine 退出。
			return errors.Errorf("receive os signal: %v", signal)
		}
	})
}
