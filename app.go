package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var httpRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total de requisições HTTP.",
	},
	[]string{"method", "endpoint"},
)
var httpRequestsPerSecond = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "http_requests_per_second",
		Help: "Requisições HTTP por segundo.",
	},
	[]string{"method", "endpoint"},
)

func initTracer() func() {
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("http-server"),
		)),
	)

	otel.SetTracerProvider(tp)

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpRequestsPerSecond)
}

func main() {
	shutdownTracer := initTracer()
	defer shutdownTracer()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	mux := http.NewServeMux()
	handleFunc := func(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
		handler := otelhttp.WithRouteTag(pattern, http.HandlerFunc(handlerFunc))
		mux.Handle(pattern, handler)
	}
	handleFunc("GET /api/v1/", func(w http.ResponseWriter, r *http.Request) {
		httpRequests.WithLabelValues(r.Method, r.URL.Path).Inc()
		tr := otel.Tracer("http-handler")
		_, span := tr.Start(r.Context(), "http-request")
		startTime := time.Now()
		defer span.End()
		success := struct {
			Message string `json:"message"`
		}{
			Message: "success",
		}
		json.NewEncoder(w).Encode(&success)
		duration := time.Since(startTime).Seconds()
		httpRequestsPerSecond.WithLabelValues(r.Method, r.URL.Path).Set(1 / duration)
	})
	mux.Handle("/metrics/", promhttp.Handler())
	srv := &http.Server{
		Addr:         "0.0.0.0:5000",
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      mux,
	}
	srv.ListenAndServe()
}
