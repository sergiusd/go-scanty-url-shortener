package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"time"
)

var buckets = []float64{0.01, 0.1, 0.5, 1, 2, 5, 10, 30, 60}

const prefix = "shortener_"

var Registry = prometheus.NewRegistry()

var (
	GenerateHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: prefix,
		Name:      "generate_histogram",
		Buckets:   buckets,
	})
	ViewCacheHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   prefix,
		Name:        "view_histogram",
		ConstLabels: map[string]string{"cache": "true"},
		Buckets:     buckets,
	})
	ViewNoCacheHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   prefix,
		Name:        "view_histogram",
		ConstLabels: map[string]string{"cache": "false"},
		Buckets:     buckets,
	})
)

func init() {
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{Namespace: prefix}))
	Registry.MustRegister(collectors.NewGoCollector())
	Registry.MustRegister(
		GenerateHistogram,
		ViewCacheHistogram,
		ViewNoCacheHistogram,
	)
}

func StartHistogramTimer(histogram prometheus.Histogram) func() {
	start := time.Now()
	stopFn := func() {
		duration := time.Now().Sub(start).Seconds()
		histogram.Observe(duration)
	}
	return stopFn
}

func StartHistogramFactoryTimer() func(prometheus.Histogram) {
	start := time.Now()
	return func(histogram prometheus.Histogram) {
		duration := time.Now().Sub(start).Seconds()
		histogram.Observe(duration)
	}
}
