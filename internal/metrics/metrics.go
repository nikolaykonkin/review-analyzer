// Пакет metrics содержит сборщики метрик Prometheus для сервиса
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics хранит реестр и все кастомные метрики
type Metrics struct {
	Registry *prometheus.Registry

	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	GigaChatProcessedTotal *prometheus.CounterVec
	GigaChatDuration       prometheus.Histogram
}

// New создает и регистрирует все метрики
func New() *Metrics {
	reg := prometheus.NewRegistry()

	// Стандартные метрики Go и процесса
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		Registry: reg,
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Общее количество HTTP-запросов.",
			},
			[]string{"method", "route", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Длительность HTTP-запросов в секундах.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),
		GigaChatProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gigachat_reviews_processed_total",
				Help: "Количество отзывов, обработанных фоновым воркером.",
			},
			[]string{"status"},
		),
		GigaChatDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "gigachat_request_duration_seconds",
				Help:    "Длительность запроса к GigaChat в секундах.",
				Buckets: []float64{1, 2, 5, 10, 20, 30, 60},
			},
		),
	}

	reg.MustRegister(m.HTTPRequestsTotal)
	reg.MustRegister(m.HTTPRequestDuration)
	reg.MustRegister(m.GigaChatProcessedTotal)
	reg.MustRegister(m.GigaChatDuration)

	return m
}
