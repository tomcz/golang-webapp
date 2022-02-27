package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

func main() {
	if err := realMain(); err != nil {
		log.Fatalln("application failed - error is:", err)
	}
	log.Println("application stopped")
}

func realMain() error {
	env := osLookupEnv("ENV", "dev")
	addr := osLookupEnv("LISTEN_ADDR", ":3000")
	cookieAuth := osLookupEnv("COOKIE_AUTH_KEY", "")
	cookieEnc := osLookupEnv("COOKIE_ENC_KEY", "")
	cookieName := osLookupEnv("COOKIE_NAME", "example")
	tlsCertFile := osLookupEnv("TLS_CERT_FILE", "")
	tlsKeyFile := osLookupEnv("TLS_KEY_FILE", "")
	traceFile := osLookupEnv("TRACE_LOG_FILE", "target/traces.jsonl")

	log.Printf("starting application version %s in %s environment\n", build.Version(), env)

	fp, err := os.Create(traceFile)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", traceFile, err)
	}
	defer fp.Close()

	log.Println("writing otel traces to", traceFile)

	tp, err := newTraceProvider(fp, env)
	if err != nil {
		return fmt.Errorf("failed to create trace provider: %w", err)
	}
	defer shutdown("trace provider", tp.Shutdown)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	isDev := env == "dev"
	session := newSessionStore(cookieAuth, cookieEnc, cookieName)
	handler := withMiddleware(newHandler(session), isDev)
	server := &http.Server{Addr: addr, Handler: handler}

	group, ctx := errgroup.WithContext(context.Background())
	group.Go(func() error {
		var err error
		if tlsCertFile != "" && tlsKeyFile != "" {
			log.Println("starting server with TLS on", addr)
			err = server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
		} else {
			log.Println("starting server without TLS on", addr)
			err = server.ListenAndServe()
		}
		if errors.Is(err, http.ErrServerClosed) {
			log.Println("server stopped")
			return nil
		}
		return err
	})
	group.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			log.Println("shutdown received")
			shutdown("server", server.Shutdown)
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

func newTraceProvider(w io.Writer, env string) (*trace.TracerProvider, error) {
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

func shutdown(src string, fn func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := fn(ctx); err != nil {
		log.Printf("unclean %s shutdown: %v\n", src, err)
	}
}
