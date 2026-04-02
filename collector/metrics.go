package collector

import (
	"net/http"
	"sync"
	"time"

	"hdp-disk-inspect/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registerCollectorsOnce sync.Once

// MetricsHandler returns an HTTP handler that exposes Prometheus metrics on /metrics.
func MetricsHandler() http.Handler {
	registerCollectorsOnce.Do(func() {
		RegisterLLDPCollector()
		RegisterNetworkCollector()
		RegisterDiskCollector()
	})
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

// StartMetricsServer starts an HTTP server on the specified address and exposes
// Prometheus metrics on /metrics.
func StartMetricsServer(addr string) {
	if addr == "" {
		return
	}

	handler := MetricsHandler()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		utils.Logger.WithField("metrics_addr", addr).Info("starting Prometheus metrics server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Logger.WithError(err).Fatal("metrics server failed")
		}
	}()
}
