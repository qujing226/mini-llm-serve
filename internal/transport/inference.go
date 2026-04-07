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
	appErrors "github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/handler"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
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
	metrics          metrics.Metrics
}

type InferenceHTTPServer struct {
	Server *http.Server
}

func NewLLMServingServer(l *zap.SugaredLogger, serverConf *conf.Conf, e handler.InferenceHandler, metrics metrics.Metrics) *InferenceHTTPServer {
	mux := http.NewServeMux()
	svc := &inferenceService{
		l:                l,
		InferenceHandler: e,
		metrics:          metrics,
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

func StartServer(lc fx.Lifecycle, i *InferenceHTTPServer, a *AdminServer, logger *zap.SugaredLogger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			iListener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", i.Server.Addr)
			if err != nil {
				logger.Errorw("[inference] failed to listen", "addr", i.Server.Addr, "err", err)
				return err
			}
			logger.Infof("[inference] listening on %s", i.Server.Addr)

			aListener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.Server.Addr)
			if err != nil {
				logger.Errorw("[admin] failed to listen", "addr", a.Server.Addr, "err", err)
				return err
			}
			logger.Infof("[admin] listening on %s", a.Server.Addr)

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Infow("recovered from panic", "err", r)
					}
				}()
				if serErr := i.Server.Serve(iListener); serErr != nil &&
					!errors.Is(serErr, http.ErrServerClosed) {
					logger.Errorw("[inference] failed to serve", "addr", i.Server.Addr, "err", serErr)
				}
			}()

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Infow("recovered from panic", "err", r)
					}
				}()
				if serErr := a.Server.Serve(aListener); serErr != nil &&
					!errors.Is(serErr, http.ErrServerClosed) {
					logger.Errorw("[admin] failed to serve", "addr", a.Server.Addr, "err", serErr)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {

			logger.Info("shutting down [inference] Server...")
			if err := i.Server.Shutdown(ctx); err != nil {
				logger.Errorw("failed to shutdown [inference] Server", "err", err)
				return err
			}
			logger.Info("shutting down [admin] Server...")
			if err := a.Server.Shutdown(ctx); err != nil {
				logger.Errorw("failed to shutdown [admin] Server", "err", err)
				return err
			}
			logger.Info("Server shutdown gracefully")
			return nil
		},
	})
}

func (i *inferenceService) Generate(ctx context.Context, request *v1.GenerateRequest) (*v1.GenerateResponse, error) {
	var (
		status           = "ok"
		executorId       = "unknown"
		requestStartTime = time.Now()
	)
	defer func() {
		requestEndTime := time.Now()
		i.metrics.ObserveRequestDuration(requestEndTime.Sub(requestStartTime).Seconds())
		i.metrics.IncRequest(status, executorId)
	}()

	req, err := model.ProtoMsgToModel(request)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return nil, appErrors.ToConnectError(err)
	}
	out, err := i.InferenceHandler.Generate(ctx, req)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return nil, appErrors.ToConnectError(err)
	}
	executorId = out.ExecutorId

	resp, err := model.ModelToProtoMsg(out)
	if err != nil {
		status = string(appErrors.CodeOf(err))
		return nil, appErrors.ToConnectError(err)
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
