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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"golang.org/x/sync/errgroup"

	"github.com/tomcz/golang-webapp/build"
)

var (
	env string
	log logrus.FieldLogger
)

func init() {
	env = osLookupEnv("ENV", "dev")
	log = logrus.WithFields(logrus.Fields{
		"build": build.Version(),
		"env":   env,
	})
}

func main() {
	if err := realMain(); err != nil {
		log.WithError(err).Fatal("application failed")
	}
	log.Info("application stopped")
}

func realMain() error {
	addr := osLookupEnv("LISTEN_ADDR", ":3000")
	cookieAuth := osLookupEnv("COOKIE_AUTH_KEY", "")
	cookieEnc := osLookupEnv("COOKIE_ENC_KEY", "")
	cookieName := osLookupEnv("COOKIE_NAME", "example")
	tlsCertFile := osLookupEnv("TLS_CERT_FILE", "")
	tlsKeyFile := osLookupEnv("TLS_KEY_FILE", "")
	traceFile := osLookupEnv("TRACE_LOG_FILE", "target/traces.jsonl")

	log.Info("starting application")

	fp, err := os.Create(traceFile)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", traceFile, err)
	}
	defer closeCleanly(traceFile, fp.Close)

	log.WithField("file", traceFile).Info("otel traces will be written to a file")

	tp, err := newTraceProvider(fp)
	if err != nil {
		return fmt.Errorf("failed to create trace provider: %w", err)
	}
	defer closeWithTimeout("trace provider", tp.Shutdown)

	otel.SetLogger(oll.New(log.WithField("component", "otel")))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	session := newSessionStore(cookieName, cookieAuth, cookieEnc)
	handler := withMiddleware(newHandler(session), env == "dev")
	server := &http.Server{Addr: addr, Handler: handler}

	group, ctx := errgroup.WithContext(context.Background())
	group.Go(func() error {
		var err error
		ll := log.WithField("addr", addr)
		if tlsCertFile != "" && tlsKeyFile != "" {
			ll.Info("starting server with TLS")
			err = server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
		} else {
			ll.Info("starting server without TLS")
			err = server.ListenAndServe()
		}
		if errors.Is(err, http.ErrServerClosed) {
			ll.Info("server stopped")
			return nil
		}
		return err
	})
	group.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			log.Info("shutdown received")
			closeWithTimeout("server", server.Shutdown)
			return nil
		case <-ctx.Done():
			return nil
		}
	})
	return group.Wait()
}

func osLookupEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
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

func closeWithTimeout(src string, fn func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := fn(ctx); err != nil {
		log.WithError(err).WithField("src", src).Error("unclean close")
	}
}

func closeCleanly(src string, fn func() error) {
	if err := fn(); err != nil {
		log.WithError(err).WithField("src", src).Error("unclean close")
	}
}
