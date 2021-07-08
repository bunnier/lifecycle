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
	ctx             context.Context  // app的核心上下文。
	cancel          func()           // 用于协调goroutine的退出。
	logger          *log.Logger      // 内部日志。
	httpServers     []*http.Server   // 维护的http服务。
	grpcServerInfos []GrpcServerInfo // 维护的grpc服务。
	errGroup        *errgroup.Group  // 用于协调goroutine。
}

// App 的选项函数。
type AppOption func(app *App)

// NewApp 返回一个App对象。
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
	// 这里直接用errGroup的ctx来做上下文控制，由于errGroup的上下文是个子上下文，app上挂着的cancel也能取消它。
	app.errGroup, app.ctx = errgroup.WithContext(app.ctx)

	return app
}

// Start 用来开启服务。
func (a *App) Run() error {
	a.startListenSystemSignal() // 开启系统信号监听。
	a.startHttpServers()        // 开启托管的Http服务。
	a.startGrpcServers()        // 开启托管的Grpc服务。
	return a.errGroup.Wait()    // 等待errGroup的结束信号。
}

// Stop 用来关闭服务。
func (a *App) Stop() error {
	a.logger.Println("start to stop app...")
	a.cancel()
	// 由于上一行的cancel联动了goroutine中的shutdown等，所以这里的Wait一定能很快返回～
	return a.errGroup.Wait()
}

// startHttpServers 用来开启托管的服务。
func (a *App) startHttpServers() {
	// 为了处理闭包问题，包一层。
	wrapServerStart := func(srv *http.Server) func() error {
		// 每一个服务起一个goroutine来监听shutdown信号。
		go func() {
			<-a.ctx.Done() // 上下文对象被取消后，各个服务就都自行了结了吧～
			a.logger.Printf("start to shutdown http server: %v\n", srv.Addr)
			srv.Shutdown(context.TODO())
		}()

		return func() error {
			a.logger.Printf("start http server: %v\n", srv.Addr)
			// 正式开启服务，阻塞方法，shutdown后这个方法才会返回。
			return errors.WithMessagef(srv.ListenAndServe(), "http server exit: %s", srv.Addr)
		}
	}

	// 在errGroup中起goroutine开始服务。
	for _, srv := range a.httpServers {
		a.errGroup.Go(wrapServerStart(srv))
	}
}

// startHttpServers 用来开启托管的服务。
func (a *App) startGrpcServers() {
	// 为了处理闭包问题，包一层。
	wrapServerStart := func(grpcServerInfo GrpcServerInfo) func() error {
		// 每一个服务起一个goroutine来监听shutdown信号。
		go func() {
			<-a.ctx.Done() // 上下文对象被取消后，各个服务就都自行了结了吧～
			a.logger.Printf("start to shutdown grpc server: %v\n", grpcServerInfo.EndPoint)
			grpcServerInfo.GrpcServer.GracefulStop()
		}()

		return func() error {
			a.logger.Printf("start grpc server: %v\n", grpcServerInfo.EndPoint)

			tcp, err := net.Listen("tcp", grpcServerInfo.EndPoint)
			if err != nil {
				return errors.Wrap(err, "grpc startup: tcp")
			}

			// 正式开启服务，阻塞方法，shutdown后这个方法才会返回。
			return errors.WithMessagef(grpcServerInfo.GrpcServer.Serve(tcp), "grpc server exit: %s", grpcServerInfo.EndPoint)
		}
	}

	// 在errGroup中起goroutine开始服务。
	for _, srv := range a.grpcServerInfos {
		a.errGroup.Go(wrapServerStart(srv))
	}
}

// startListenSystemSignal 开启系统信号监听。
func (a *App) startListenSystemSignal() {
	// 在errGroup中起一个用于监听系统信号的goroutine。
	a.errGroup.Go(func() error {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM) // 把系统信号注册到signalCh中。

		select {
		case <-a.ctx.Done():
			signal.Stop(signalCh) // 停止系统信号的监听。
			close(signalCh)
			return a.ctx.Err()
		case signal := <-signalCh:
			// 这边返回error后，errGroup会调用context的cancel，令其它的goroutine退出。
			return errors.Errorf("receive os signal: %v", signal)
		}
	})
}
