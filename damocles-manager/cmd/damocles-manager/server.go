package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/dtynn/dix"
	"github.com/ipfs-force-community/damocles/damocles-manager/core"
	"github.com/ipfs-force-community/damocles/damocles-manager/metrics"
	"github.com/ipfs-force-community/damocles/damocles-manager/metrics/proxy"
	managerplugin "github.com/ipfs-force-community/damocles/manager-plugin"
)

func NewAPIService(sealerAPI core.SealerAPI, minerAPI core.MinerAPI, plugins *managerplugin.LoadedPlugins) *APIService {
	return &APIService{
		sealerAPI: sealerAPI,
		minerAPI:  minerAPI,
		plugins:   plugins,
	}
}

type handler struct {
	namespace string
	hdl       interface{}
}

type APIService struct {
	sealerAPI core.SealerAPI
	minerAPI  core.MinerAPI
	plugins   *managerplugin.LoadedPlugins
}

func (api *APIService) handlers() []handler {
	handlers := make([]handler, 0, 2)
	handlers = append(handlers, handler{
		namespace: core.SealerAPINamespace,
		hdl:       api.sealerAPI,
	})
	handlers = append(handlers, handler{
		namespace: core.MinerAPINamespace,
		hdl:       api.minerAPI,
	})
	if api.plugins != nil {
		_ = api.plugins.Foreach(managerplugin.RegisterJsonRpc, func(plugin *managerplugin.Plugin) error {
			m := managerplugin.DeclareRegisterJsonRpcManifest(plugin.Manifest)
			namespace, hdl := m.Handler()
			log.Infof("register json rpc handler by plugin(%s). namespace: '%s'", plugin.Name, namespace)
			handlers = append(handlers, handler{
				namespace: namespace,
				hdl:       hdl,
			})
			return nil
		})
	}
	return handlers
}

func serveAPI(ctx context.Context, stopper dix.StopFunc, apiService *APIService, addr string) error {
	mux, err := buildRPCServer(apiService)
	if err != nil {
		return fmt.Errorf("construct rpc server: %w", err)
	}

	// register piece store proxy

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	errCh := make(chan error, 1)
	go func() {
		log.Infof("trying to listen on %s", httpServer.Addr)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http server error: %w", err)
		}
	}()

	log.Info("daemon running")
	select {
	case <-ctx.Done():
		log.Warn("process signal captured")

	case e := <-errCh:
		log.Errorf("error occurred: %s", e)
	}

	log.Info("stop application")
	stopper(context.Background()) // nolint: errcheck

	log.Info("http server shutdown")
	if err := httpServer.Shutdown(context.Background()); err != nil {
		log.Errorf("shutdown http server: %s", err)
	}

	_ = log.Sync()
	return nil
}

func buildRPCServer(apiService *APIService, opts ...jsonrpc.ServerOption) (*http.ServeMux, error) {
	// use field
	opts = append(opts, jsonrpc.WithProxyBind(jsonrpc.PBField))
	server := jsonrpc.NewServer(opts...)

	for _, hdl := range apiService.handlers() {
		server.Register(hdl.namespace, proxy.MetricedAPI(hdl.namespace, hdl.hdl))
	}

	http.Handle(fmt.Sprintf("/rpc/v%d", core.MajorVersion), server)

	// metrics
	http.Handle("/metrics", metrics.Exporter())
	return http.DefaultServeMux, nil
}
