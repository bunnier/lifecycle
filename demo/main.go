package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/bunnier/lifecycle"
	"google.golang.org/grpc"
)

func main() {
	// 服务1：grpc
	grpcServer := lifecycle.NewGrpcServerInfo(grpc.NewServer(), "127.0.0.1:8081")
	// 服务2：http
	httpServer1 := &http.Server{
		Addr:    "127.0.0.1:8082",
		Handler: nil,
	}
	// 服务3：http
	httpServer2 := &http.Server{
		Addr:    "127.0.0.1:8083",
		Handler: nil,
	}

	// 托管上面 3 个服务，一起启停。
	app := lifecycle.NewApp(
		lifecycle.WithGrpcServer(grpcServer),
		lifecycle.WithHttpServer(httpServer1),
		lifecycle.WithHttpServer(httpServer2),
		lifecycle.WithLog(log.Default()),
	)

	time.AfterFunc(time.Second*5, func() {
		app.Stop() // 5s 后退出所有服务。
	})

	// 开始服务。
	if err := app.Run(); err != nil {
		log.Println(err)
	}

	fmt.Println("App exited.")
}
