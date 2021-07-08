package lifecycle

import "google.golang.org/grpc"

// GrpcServerInfo 存放Grpc服务信息。
type GrpcServerInfo struct {
	GrpcServer *grpc.Server
	EndPoint   string
}

// NewGrpcServerInfo 用于创建供App托管生命周期的Grpc服务信息。
func NewGrpcServerInfo(grpcServer *grpc.Server, endPoint string) GrpcServerInfo {
	return GrpcServerInfo{
		GrpcServer: grpcServer,
		EndPoint:   endPoint,
	}
}

// WithGrpcServer 用于构建向App注册Server的选项函数。
func WithGrpcServer(grpcServerInfo GrpcServerInfo) AppOption {
	return func(app *App) {
		app.grpcServerInfos = append(app.grpcServerInfos, grpcServerInfo)
	}
}
