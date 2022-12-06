package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	oll "github.com/bombsimon/logrusr/v2"
	"github.com/sirupsen/logrus"
	"github.com/tomcz/gotools/env"
	"github.com/tomcz/gotools/errgroup"
	"github.com/tomcz/gotools/quiet"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"

	"github.com/tomcz/golang-webapp/build"
)

const quietTimeout = 100 * time.Millisecond

var log logrus.FieldLogger

func init() {
	log = logrus.WithField("build", build.Version())
}

func main() {
	if err := realMain(); err != nil {
		log.WithError(err).Fatal("application failed")
	}
	log.Info("application stopped")
}

type appConfig struct {
	Addr        string `mapstructure:"LISTEN_ADDR"`
	CookieAuth  string `mapstructure:"COOKIE_AUTH_KEY"`
	CookieEnc   string `mapstructure:"COOKIE_ENC_KEY"`
	CookieName  string `mapstructure:"COOKIE_NAME"`
	TlsCertFile string `mapstructure:"TLS_CERT_FILE"`
	TlsKeyFile  string `mapstructure:"TLS_KEY_FILE"`
	TraceFile   string `mapstructure:"TRACE_LOG_FILE"`
	Environment string `mapstructure:"ENV"`
}

func realMain() error {
	cfg := appConfig{
		Addr:        ":3000",
		CookieName:  "example",
		TraceFile:   "target/traces.jsonl",
		Environment: "development",
	}
	if err := env.PopulateFromEnv(&cfg); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	log = log.WithField("env", cfg.Environment)
	log.Info("starting application")

	fp, err := os.Create(cfg.TraceFile)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", cfg.TraceFile, err)
	}
	defer quiet.Close(fp)

	log.WithField("file", cfg.TraceFile).Info("otel traces will be written to a file")

	tp, err := newTraceProvider(fp, cfg.Environment)
	if err != nil {
		return fmt.Errorf("failed to create trace provider: %w", err)
	}
	defer quiet.CloseWithTimeout(tp.Shutdown, quietTimeout)

	otel.SetLogger(oll.New(log.WithField("component", "otel")))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	session := newSessionStore(cfg.CookieName, cfg.CookieAuth, cfg.CookieEnc)
	handler := withMiddleware(newHandler(session), cfg.Environment == "development")
	server := &http.Server{Addr: cfg.Addr, Handler: handler}

	group, ctx := errgroup.NewContext(context.Background())
	group.Go(func() error {
		ll := log.WithField("addr", cfg.Addr)
		if cfg.TlsCertFile != "" && cfg.TlsKeyFile != "" {
			ll.Info("starting server with TLS")
			return server.ListenAndServeTLS(cfg.TlsCertFile, cfg.TlsKeyFile)
		}
		ll.Info("starting server without TLS")
		return server.ListenAndServe()
	})
	group.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			log.Info("shutdown received")
			quiet.CloseWithTimeout(server.Shutdown, quietTimeout)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	err = group.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newTraceProvider(w io.Writer, environment string) (*trace.TracerProvider, error) {
	tw, err := stdouttrace.New(stdouttrace.WithWriter(w))
	if err != nil {
		return nil, err
	}
	tr, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("golang-webapp"),
			semconv.ServiceVersionKey.String(build.Version()),
			attribute.String("environment", environment),
		),
	)
	if err != nil {
		return nil, err
	}
	return trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(tr),
		trace.WithBatcher(tw),
	), nil
}
