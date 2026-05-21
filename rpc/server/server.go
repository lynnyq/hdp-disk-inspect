package server

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lynnyq/hdp-disk-inspect/rpc/auth"
	pb "github.com/lynnyq/hdp-disk-inspect/rpc/proto"
	"github.com/lynnyq/hdp-disk-inspect/utils"

	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type Server struct {
	pb.UnimplementedTaskServer
}

var keepAlivePolicy = keepalive.EnforcementPolicy{
	MinTime:             10 * time.Second,
	PermitWithoutStream: true,
}

var keepAliveParams = keepalive.ServerParameters{
	MaxConnectionIdle: 30 * time.Second,
	Time:              30 * time.Second,
	Timeout:           3 * time.Second,
}

func (s Server) Run(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			utils.Logger.Error(err)
		}
	}()
	utils.Logger.Infof("execute cmd start: [id: %d cmd: %s]", req.Id, req.Command)
	result, err := utils.ExecShell(ctx, req.Command)
	resp := new(pb.TaskResponse)

	resp.StartTime = result.StartTime.UnixMilli()
	resp.EndTime = result.EndTime.UnixMilli()
	resp.DurationMs = result.DurationMs()
	resp.Output = result.Stdout
	resp.Stderr = result.Stderr
	resp.ExitCode = int32(result.ExitCode)

	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Error = ""
	}

	utils.Logger.Infof("execute cmd end: [id: %d cmd: %s exit_code: %d duration_ms: %d]",
		req.Id, req.Command, resp.ExitCode, resp.DurationMs)

	return resp, nil
}

func Start(addr string, enableTLS bool, certificate auth.Certificate, httpHandler http.Handler) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		utils.Logger.Fatal(err)
	}
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepAliveParams),
		grpc.KeepaliveEnforcementPolicy(keepAlivePolicy),
	}
	if enableTLS && httpHandler == nil {
		tlsConfig, err := certificate.GetTLSConfigForServer()
		if err != nil {
			utils.Logger.Fatal(err)
		}
		opt := grpc.Creds(credentials.NewTLS(tlsConfig))
		opts = append(opts, opt)
	}
	server := grpc.NewServer(opts...)
	pb.RegisterTaskServer(server, Server{})
	utils.Logger.Infof("server listen on %s", addr)

	if httpHandler == nil {
		go func() {
			err = server.Serve(l)
			if err != nil {
				utils.Logger.Fatal(err)
			}
		}()
	} else {
		mux := http.NewServeMux()
		mux.Handle("/metrics", httpHandler)
		httpServer := &http.Server{
			Handler:           grpcHandlerFunc(server, mux),
			ReadHeaderTimeout: 5 * time.Second,
		}
		if enableTLS {
			tlsConfig, err := certificate.GetTLSConfigForServer()
			if err != nil {
				utils.Logger.Fatal(err)
			}
			httpServer.TLSConfig = tlsConfig
			go func() {
				err = httpServer.Serve(tls.NewListener(l, tlsConfig))
				if err != nil {
					utils.Logger.Fatal(err)
				}
			}()
		} else {
			h2s := &http2.Server{}
			httpServer.Handler = h2c.NewHandler(httpServer.Handler, h2s)
			go func() {
				err = httpServer.Serve(l)
				if err != nil {
					utils.Logger.Fatal(err)
				}
			}()
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	for {
		s := <-c
		utils.Logger.Infoln("收到信号 -- ", s)
		switch s {
		case syscall.SIGHUP:
			utils.Logger.Infoln("收到终端断开信号, 忽略")
		case syscall.SIGINT, syscall.SIGTERM:
			utils.Logger.Info("应用准备退出")
			server.GracefulStop()
			return
		}
	}
}

func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		otherHandler.ServeHTTP(w, r)
	})
}
