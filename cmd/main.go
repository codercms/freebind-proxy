//go:build linux

package main

import (
	"context"
	"errors"
	"flag"
	"github.com/codercms/freebind-proxy/proxy"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

var localNet string
var localIface string
var addSubnetRoute bool

var listenAddr string

var authUser string
var authPass string

var logLevel string

func init() {
	flag.StringVar(&localNet, "net", "", "Network subnet, e.g. 10.0.0.1/24")
	flag.StringVar(&localIface, "iface", "eth0", "Local interface to bind to")
	flag.BoolVar(&addSubnetRoute, "add-route", false, "Add route to network subnet")

	flag.StringVar(&listenAddr, "addr", ":8080", "Listen address")

	flag.StringVar(&authUser, "auth-user", "", "Authentication user (HTTP basic)")
	flag.StringVar(&authPass, "auth-pass", "", "Authentication password (HTTP basic)")

	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, fatal)")
}

func main() {
	flag.Parse()

	_, ipNet, err := net.ParseCIDR(strings.TrimSpace(localNet))
	if err != nil {
		log.Fatal("Failed to parse subnet", err)
	}

	var options []proxy.Option
	var logger *zap.Logger

	if len(logLevel) > 0 {
		logLvl, err := zap.ParseAtomicLevel(logLevel)
		if err != nil {
			log.Fatal("Failed to parse log level", err)
		}

		logCfg := zap.NewProductionConfig()
		logCfg.Level = logLvl
		logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		logger, err = logCfg.Build()
		if err != nil {
			log.Fatal("Failed to build logger", err)
		}

		options = append(options, proxy.WithLogger(logger))
	} else {
		logger, err = zap.NewProduction()
		if err != nil {
			log.Fatal("Failed to create default logger", err)
		}
	}

	logger.Info("Using subnet", zap.String("subnet", ipNet.String()))

	if addSubnetRoute {
		cmd := exec.Command("ip", "route", "add", "local", ipNet.String(), "dev", localIface)

		logger.Info("Adding ip subnet route", zap.String("cmd", cmd.String()))

		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() != 0 {
				output, err := cmd.Output()
				var outputStr string
				if err == nil {
					outputStr = string(output)
				}

				logger.Fatal("Failed to add route to network subnet",
					zap.Int("exitCode", exitErr.ExitCode()),
					zap.String("output", outputStr),
				)
			} else {
				logger.Fatal("Failed to run add route cmd", zap.Error(err))
			}
		}
	}

	dialerFactory := proxy.MakeRandIpDialerFactory(ipNet)

	if len(listenAddr) > 0 {
		options = append(options, proxy.WithListenAddr(listenAddr))
	}

	if len(authUser) > 0 && len(authPass) > 0 {
		logger.Info("Using basic authentication")

		options = append(options, proxy.WithAuthFunc(func(usr, passwd string) bool {
			return usr == authUser && passwd == authPass
		}))
	}

	server := proxy.MakeServer(dialerFactory, options...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		defer close(sigCh)

		select {
		case sig := <-sigCh:
			logger.Info("Received stop signal, shutting down...", zap.String("signal", strings.ToUpper(sig.String())))

			cancel()
		case <-ctx.Done():
			return
		}
	}()

	if err := server.Run(ctx); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Failed to start/stop server", zap.Error(err))
		}
	}

	logger.Info("Server stopped")
}
