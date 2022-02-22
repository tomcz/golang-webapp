package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	env := flag.String("env", "dev", "Environment name")
	addr := flag.String("addr", ":3000", "Listen address")
	cookieAuth := flag.String("cookie-auth", "", "Cookie authentication key")
	cookieEnc := flag.String("cookie-enc", "", "Cookie encryption key")
	cookieName := flag.String("cookie-name", "example", "Cookie name")
	tlsCertFile := flag.String("tls-cert", "", "TLS certificate file")
	tlsKeyFile := flag.String("tls-key", "", "TLS private key file")
	traceFile := flag.String("trace-out", "target/traces.jsonl", "trace output file")
	flag.Parse()

	fp, err := os.Create(*traceFile)
	if err != nil {
		log.Fatalf("failed to create %s - error is: %v\n", *traceFile, err)
	}
	log.Println("writing otel traces to", *traceFile)

	tp, err := newTraceProvider(fp, *env)
	if err != nil {
		fp.Close() // fatal logs bork defer
		log.Fatalln("failed to create trace provider - error is:", err)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	isDev := *env == "dev"
	session := newSessionStore(*cookieAuth, *cookieEnc, *cookieName)
	handler := withMiddleware(newHandler(session, isDev), isDev)
	server := &http.Server{Addr: *addr, Handler: handler}

	group, ctx := errgroup.WithContext(context.Background())
	group.Go(func() error {
		var err error
		if *tlsCertFile != "" && *tlsKeyFile != "" {
			log.Println("starting server with TLS on", *addr)
			err = server.ListenAndServeTLS(*tlsCertFile, *tlsKeyFile)
		} else {
			log.Println("starting server without TLS on", *addr)
			err = server.ListenAndServe()
		}
		if errors.Is(err, http.ErrServerClosed) {
			log.Println("server stopped")
			return nil
		}
		return err
	})
	group.Go(func() error {
		defer func() {
			tp.Shutdown(context.Background())
			fp.Close()
		}()
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-signalChan:
			log.Println("shutdown received")
			return server.Shutdown(context.Background())
		case <-ctx.Done():
			return nil
		}
	})
	if err = group.Wait(); err != nil {
		log.Fatalln("application failed - error is:", err)
	}
	log.Println("application stopped")
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
