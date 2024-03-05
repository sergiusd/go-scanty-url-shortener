package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"time"
)

var buckets = []float64{0.01, 0.1, 0.5, 1, 2, 5, 10, 30, 60}

var (
	GenerateHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "generate_histogram",
		Buckets: buckets,
	})
	ViewCacheHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:        "view_histogram",
		ConstLabels: map[string]string{"cache": "true"},
		Buckets:     buckets,
	})
	ViewNoCacheHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:        "view_histogram",
		ConstLabels: map[string]string{"cache": "false"},
		Buckets:     buckets,
	})
)

func StartHistogramTimer(histogram prometheus.Histogram) func() {
	start := time.Now().UnixMilli()
	stopFn := func() {
		duration := float64(time.Now().UnixMilli()-start) / 1000
		histogram.Observe(duration)
	}
	return stopFn
}

func StartHistogramFactoryTimer() func(prometheus.Histogram) {
	start := time.Now().UnixMilli()
	return func(histogram prometheus.Histogram) {
		duration := float64(time.Now().UnixMilli()-start) / 1000
		histogram.Observe(duration)
	}
}
