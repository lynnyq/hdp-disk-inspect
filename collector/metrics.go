package collector

import (
	"net/http"
	"sync"

	"github.com/lynnyq/hdp-disk-inspect/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registerCollectorsOnce sync.Once
var metricsRegistry *prometheus.Registry

func MetricsHandler() http.Handler {
	registerCollectorsOnce.Do(func() {
		metricsRegistry = prometheus.NewRegistry()
		metricsRegistry.MustRegister(newLLDPCollector())
		metricsRegistry.MustRegister(newNetworkCollector())
		metricsRegistry.MustRegister(newDiskCollector())
	})
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	}))
	return mux
}

func MetricsHandlerWithGoMetrics() http.Handler {
	registerCollectorsOnce.Do(func() {
		RegisterLLDPCollector()
		RegisterNetworkCollector()
		RegisterDiskCollector()
	})
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func StartMetricsServer(addr string) {
	if addr == "" {
		return
	}

	handler := MetricsHandler()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5,
	}

	go func() {
		utils.Logger.WithField("metrics_addr", addr).Info("starting Prometheus metrics server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Logger.WithError(err).Fatal("metrics server failed")
		}
	}()
}
