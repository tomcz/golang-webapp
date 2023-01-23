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
	"github.com/tomcz/golang-webapp/internal"
)

const (
	development  = "development"
	quietTimeout = 100 * time.Millisecond
)

var (
	env string
	log logrus.FieldLogger
)

func init() {
	env = getenv("ENV", development)
	if env == development {
		logrus.SetFormatter(&logrus.TextFormatter{})
	} else {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	log = logrus.WithFields(logrus.Fields{
		"component": "main",
		"build":     build.Version(),
		"env":       env,
	})
}

func main() {
	if err := realMain(); err != nil {
		log.WithError(err).Fatal("application failed")
	}
	log.Info("application stopped")
}

func realMain() error {
	log.Info("starting application")

	addr := getenv("LISTEN_ADDR", ":3000")
	cookieAuth := getenv("COOKIE_AUTH_KEY", "")
	cookieEnc := getenv("COOKIE_ENC_KEY", "")
	cookieName := getenv("COOKIE_NAME", "example")
	tlsCertFile := getenv("TLS_CERT_FILE", "")
	tlsKeyFile := getenv("TLS_KEY_FILE", "")
	traceFile := getenv("TRACE_LOG_FILE", "target/traces.jsonl")
	withTLS := tlsCertFile != "" && tlsKeyFile != ""

	fp, err := os.Create(traceFile)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", traceFile, err)
	}
	defer quiet.Close(fp)

	log.WithField("file", traceFile).Info("otel traces will be written to a file")

	tp, err := newTraceProvider(fp)
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

	session := internal.NewSessionStore(cookieName, cookieAuth, cookieEnc)
	handler := internal.WithMiddleware(internal.NewHandler(session), withTLS, log)
	server := &http.Server{Addr: addr, Handler: handler}

	group, ctx := errgroup.NewContext(context.Background())
	group.Go(func() error {
		ll := log.WithField("addr", addr)
		if withTLS {
			ll.Info("starting server with TLS")
			return server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
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
			return nil
		}
	})
	err = group.Wait()
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newTraceProvider(w io.Writer) (*trace.TracerProvider, error) {
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
			attribute.String("environment", env),
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

func getenv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}
