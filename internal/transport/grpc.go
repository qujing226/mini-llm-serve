package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/cockroachdb/errors"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1/mini_llm_servev1connect"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/handler"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"go.uber.org/fx"
	"go.uber.org/zap"
	brotli "go.withmatt.com/connect-brotli"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	ServerShutdownTimeout = 30 * time.Second
	CompressionMinBytes   = 1024 // 1KB
)

type inferenceService struct {
	l                *zap.SugaredLogger
	InferenceHandler handler.InferenceHandler
}

type InferenceHTTPServer struct {
	Server *http.Server
}

func NewLLMServingServer(l *zap.SugaredLogger, serverConf *conf.Conf, e handler.InferenceHandler) *InferenceHTTPServer {
	mux := http.NewServeMux()
	svc := &inferenceService{
		l:                l,
		InferenceHandler: e,
	}

	path, handler := mini_llm_servev1connect.NewInferenceServiceHandler(
		svc,
		connect.WithInterceptors(),
		connect.WithCompressMinBytes(CompressionMinBytes),
		brotli.WithCompression(),
	)
	mux.Handle(path, handler)

	h2cHandler := h2c.NewHandler(mux, &http2.Server{})

	port, err := extractPortNumber(serverConf.Server.Address)
	if err != nil {
		panic(err)
	}
	addr := fmt.Sprintf("0.0.0.0:%d", port)

	srv := &http.Server{
		Addr:                         addr,
		Handler:                      h2cHandler,
		DisableGeneralOptionsHandler: false,
		ReadTimeout:                  3 * time.Second,
		ReadHeaderTimeout:            3 * time.Second,
		WriteTimeout:                 5 * time.Second,
		IdleTimeout:                  60 * time.Second,
	}

	return &InferenceHTTPServer{
		Server: srv,
	}
}

func StartInferenceServer(lc fx.Lifecycle, i *InferenceHTTPServer, logger *zap.SugaredLogger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", i.Server.Addr)
			if err != nil {
				logger.Errorw("failed to listen", "addr", i.Server.Addr, "err", err)
				return err
			}
			logger.Infof("listening on %s", i.Server.Addr)

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Infow("recovered from panic", "err", r)
					}
				}()
				if serErr := i.Server.Serve(listener); serErr != nil &&
					!errors.Is(serErr, http.ErrServerClosed) {
					logger.Errorw("failed to serve", "addr", i.Server.Addr, "err", serErr)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shoutdownCtx, cancel := context.WithTimeout(ctx, ServerShutdownTimeout)
			defer cancel()

			logger.Info("shutting down server...")
			if err := i.Server.Shutdown(shoutdownCtx); err != nil {
				logger.Errorw("failed to shutdown server", "err", err)
				return err
			}
			logger.Info("server shutdown gracefully")
			return nil
		},
	})
}

func (i *inferenceService) Generate(ctx context.Context, request *v1.GenerateRequest) (*v1.GenerateResponse, error) {
	req, err := model.ProtoMsgToModel(request)
	if err != nil {
		return nil, err
	}
	out, err := i.InferenceHandler.Generate(ctx, req)
	if err != nil {
		return nil, err
	}
	resp, err := model.ModelToProtoMsg(out)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func extractPortNumber(addr string) (int, error) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, errors.New("invalid port")
	}
	return port, nil
}
