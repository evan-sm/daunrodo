// Package observability provides Prometheus metrics for the application.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all application metrics.
type Metrics struct {
	// Job metrics
	JobsCreated      prometheus.Counter
	JobsCompleted    prometheus.Counter
	JobsFailed       prometheus.Counter
	JobsInProgress   prometheus.Gauge
	JobDownloadBytes prometheus.Counter
	JobDuration      prometheus.Histogram

	// Storage metrics
	CleanupJobsTotal   prometheus.Counter
	CleanupFilesTotal  prometheus.Counter
	StoredJobsTotal    prometheus.Gauge
	StoredPublications prometheus.Gauge

	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPResponseSize    *prometheus.HistogramVec

	// Proxy metrics
	ProxyRequestsTotal *prometheus.CounterVec
	ProxyFailures      *prometheus.CounterVec
	ProxiesAvailable   prometheus.Gauge

	// Downloader metrics
	DownloaderRequestsTotal *prometheus.CounterVec
	DownloaderErrors        *prometheus.CounterVec

	// System metrics
	GoRoutines prometheus.Gauge
}

// New creates and registers all application metrics.
func New() *Metrics {
	metrics := &Metrics{
		// Job metrics
		JobsCreated: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "jobs",
			Name:      "created_total",
			Help:      "Total number of jobs created",
		}),
		JobsCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "jobs",
			Name:      "completed_total",
			Help:      "Total number of jobs completed successfully",
		}),
		JobsFailed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "jobs",
			Name:      "failed_total",
			Help:      "Total number of jobs that failed",
		}),
		JobsInProgress: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "daunrodo",
			Subsystem: "jobs",
			Name:      "in_progress",
			Help:      "Number of jobs currently in progress",
		}),
		JobDownloadBytes: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "jobs",
			Name:      "download_bytes_total",
			Help:      "Total bytes downloaded across all jobs",
		}),
		JobDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "daunrodo",
			Subsystem: "jobs",
			Name:      "duration_seconds",
			Help:      "Histogram of job download duration in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		}),

		// Storage metrics
		CleanupJobsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "storage",
			Name:      "cleanup_jobs_total",
			Help:      "Total number of expired jobs cleaned up",
		}),
		CleanupFilesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "storage",
			Name:      "cleanup_files_total",
			Help:      "Total number of expired files cleaned up",
		}),
		StoredJobsTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "daunrodo",
			Subsystem: "storage",
			Name:      "jobs_current",
			Help:      "Current number of stored jobs",
		}),
		StoredPublications: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "daunrodo",
			Subsystem: "storage",
			Name:      "publications_current",
			Help:      "Current number of stored publications",
		}),

		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "daunrodo",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),
		HTTPResponseSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "daunrodo",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "Histogram of HTTP response sizes in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		}, []string{"method", "path"}),

		// Proxy metrics
		ProxyRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "proxy",
			Name:      "requests_total",
			Help:      "Total number of requests made through proxies",
		}, []string{"proxy"}),
		ProxyFailures: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "proxy",
			Name:      "failures_total",
			Help:      "Total number of proxy failures",
		}, []string{"proxy"}),
		ProxiesAvailable: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "daunrodo",
			Subsystem: "proxy",
			Name:      "available",
			Help:      "Number of currently available proxies",
		}),

		// Downloader metrics
		DownloaderRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "downloader",
			Name:      "requests_total",
			Help:      "Total number of download requests",
		}, []string{"downloader", "status"}),
		DownloaderErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "daunrodo",
			Subsystem: "downloader",
			Name:      "errors_total",
			Help:      "Total number of download errors",
		}, []string{"downloader", "error_type"}),

		// System metrics
		GoRoutines: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "daunrodo",
			Subsystem: "system",
			Name:      "goroutines",
			Help:      "Number of goroutines",
		}),
	}

	return metrics
}

// Handler returns the Prometheus HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// JobTimer returns a function to record job duration.
func (m *Metrics) JobTimer() func() {
	start := time.Now()

	return func() {
		m.JobDuration.Observe(time.Since(start).Seconds())
	}
}

// RecordHTTPRequest records HTTP request metrics.
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration, size int) {
	statusStr := strconv.Itoa(status)
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	m.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(size))
}

// RecordJobCreated increments the jobs created counter.
func (m *Metrics) RecordJobCreated() {
	m.JobsCreated.Inc()
	m.JobsInProgress.Inc()
}

// RecordJobCompleted records a completed job.
func (m *Metrics) RecordJobCompleted() {
	m.JobsCompleted.Inc()
	m.JobsInProgress.Dec()
}

// RecordJobFailed records a failed job.
func (m *Metrics) RecordJobFailed() {
	m.JobsFailed.Inc()
	m.JobsInProgress.Dec()
}

// RecordCleanup records cleanup metrics.
func (m *Metrics) RecordCleanup(jobs, files int) {
	m.CleanupJobsTotal.Add(float64(jobs))
	m.CleanupFilesTotal.Add(float64(files))
}

// RecordDownloaderRequest records a download request.
func (m *Metrics) RecordDownloaderRequest(downloader, status string) {
	m.DownloaderRequestsTotal.WithLabelValues(downloader, status).Inc()
}

// RecordDownloaderError records a download error.
func (m *Metrics) RecordDownloaderError(downloader, errorType string) {
	m.DownloaderErrors.WithLabelValues(downloader, errorType).Inc()
}

// RecordProxyRequest records a proxy request.
func (m *Metrics) RecordProxyRequest(proxy string) {
	m.ProxyRequestsTotal.WithLabelValues(proxy).Inc()
}

// RecordProxyFailure records a proxy failure.
func (m *Metrics) RecordProxyFailure(proxy string) {
	m.ProxyFailures.WithLabelValues(proxy).Inc()
}

// SetProxiesAvailable sets the number of available proxies.
func (m *Metrics) SetProxiesAvailable(count int) {
	m.ProxiesAvailable.Set(float64(count))
}

// SetStoredJobs sets the number of stored jobs.
func (m *Metrics) SetStoredJobs(count int) {
	m.StoredJobsTotal.Set(float64(count))
}

// SetStoredPublications sets the number of stored publications.
func (m *Metrics) SetStoredPublications(count int) {
	m.StoredPublications.Set(float64(count))
}

// SetGoroutines sets the current goroutine count.
func (m *Metrics) SetGoroutines(count int) {
	m.GoRoutines.Set(float64(count))
}
