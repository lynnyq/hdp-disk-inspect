// Command hdp-disk-inspect
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"hdp-disk-inspect/collector"
	"hdp-disk-inspect/rpc/auth"
	"hdp-disk-inspect/rpc/server"
	"hdp-disk-inspect/utils"
)

var (
	// AppVersion holds the application version, typically set via build flags.
	// BuildDate holds the build date, typically set via build flags.
	// GitCommit holds the git commit hash, typically set via build flags.
	AppVersion, BuildDate, GitCommit string
)

func main() {
	var serverAddr string
	var allowRoot bool
	var version bool
	var CAFile string
	var certFile string
	var keyFile string
	var enableTLS bool
	var logLevel string
	flag.BoolVar(&allowRoot, "allow-root", false, "./hdp-disk-inspect -allow-root")
	flag.StringVar(&serverAddr, "s", "0.0.0.0:5921", "./hdp-disk-inspect -s ip:port")
	flag.BoolVar(&version, "v", false, "./hdp-disk-inspect -v")
	flag.BoolVar(&enableTLS, "enable-tls", false, "./hdp-disk-inspect -enable-tls")
	flag.StringVar(&CAFile, "ca-file", "", "./hdp-disk-inspect -ca-file path")
	flag.StringVar(&certFile, "cert-file", "", "./hdp-disk-inspect -cert-file path")
	flag.StringVar(&keyFile, "key-file", "", "./hdp-disk-inspect -key-file path")
	flag.StringVar(&logLevel, "log-level", "info", "-log-level error")
	// flag.StringVar(&metricsAddr, "metrics-addr", "0.0.0.0:5921", "./hdp-disk-inspect -metrics-addr ip:port")
	flag.Parse()
	if err := utils.InitLogger(logLevel); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if version {
		// goutil.PrintAppVersion(AppVersion, GitCommit, BuildDate)
		fmt.Println("v1.0")
		return
	}

	if enableTLS {
		if !utils.FileExist(CAFile) {
			utils.Logger.Fatalf("failed to read ca cert file: %s", CAFile)
		}
		if !utils.FileExist(certFile) {
			utils.Logger.Fatalf("failed to read server cert file: %s", certFile)
			return
		}
		if !utils.FileExist(keyFile) {
			utils.Logger.Fatalf("failed to read server key file: %s", keyFile)
			return
		}
	}

	certificate := auth.Certificate{
		CAFile:   strings.TrimSpace(CAFile),
		CertFile: strings.TrimSpace(certFile),
		KeyFile:  strings.TrimSpace(keyFile),
	}

	if runtime.GOOS != "windows" && os.Getuid() == 0 && !allowRoot {
		utils.Logger.Fatal("Do not run hdp-disk-inspect as root user")
		return
	}

	utils.Logger.WithField("addr", serverAddr).WithField("enable_tls", enableTLS).Info("starting server")
	server.Start(serverAddr, enableTLS, certificate, collector.MetricsHandler())

}
