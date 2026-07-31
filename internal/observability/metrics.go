// Package observability provides Prometheus metrics for the application.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsNamespace           = "daunrodo"
	metricsSubsystemJobs       = "jobs"
	metricsSubsystemStorage    = "storage"
	metricsSubsystemHTTP       = "http"
	metricsSubsystemProxy      = "proxy"
	metricsSubsystemDownloader = "downloader"
	metricsSubsystemDepManager = "depmanager"
	metricsNameRequestsTotal   = "requests_total"
	metricsLabelMethod         = "method"
	metricsLabelCode           = "code"
	metricsLabelProxy          = "proxy"
)

// Metrics holds all application metrics.
type Metrics struct {
	registry *prometheus.Registry

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
	HTTPInFlight        prometheus.Gauge
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

	// Dependency manager metrics
	DependencyBinaryDownloadsTotal *prometheus.CounterVec
}

// New creates and registers all application metrics.
func New() *Metrics {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	factory := promauto.With(registry)

	metrics := &Metrics{
		registry: registry,

		// Job metrics
		JobsCreated: factory.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemJobs,
			Name:      "created_total",
			Help:      "Total number of jobs created",
		}),
		JobsCompleted: factory.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemJobs,
			Name:      "completed_total",
			Help:      "Total number of jobs completed successfully",
		}),
		JobsFailed: factory.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemJobs,
			Name:      "failed_total",
			Help:      "Total number of jobs that failed",
		}),
		JobsInProgress: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemJobs,
			Name:      "in_progress",
			Help:      "Number of jobs currently in progress",
		}),
		JobDownloadBytes: factory.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemJobs,
			Name:      "download_bytes_total",
			Help:      "Total bytes downloaded across all jobs",
		}),
		JobDuration: factory.NewHistogram(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemJobs,
			Name:      "duration_seconds",
			Help:      "Histogram of job download duration in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		}),

		// Storage metrics
		CleanupJobsTotal: factory.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "cleanup_jobs_total",
			Help:      "Total number of expired jobs cleaned up",
		}),
		CleanupFilesTotal: factory.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "cleanup_files_total",
			Help:      "Total number of expired files cleaned up",
		}),
		StoredJobsTotal: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "jobs_current",
			Help:      "Current number of stored jobs",
		}),
		StoredPublications: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "publications_current",
			Help:      "Current number of stored publications",
		}),

		// HTTP metrics
		HTTPInFlight: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemHTTP,
			Name:      "in_flight",
			Help:      "Current number of HTTP requests being served",
		}),
		HTTPRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemHTTP,
			Name:      metricsNameRequestsTotal,
			Help:      "Total number of HTTP requests",
		}, []string{metricsLabelMethod, metricsLabelCode}),
		HTTPRequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemHTTP,
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{metricsLabelMethod, metricsLabelCode}),
		HTTPResponseSize: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemHTTP,
			Name:      "response_size_bytes",
			Help:      "Histogram of HTTP response sizes in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		}, []string{metricsLabelMethod, metricsLabelCode}),

		// Proxy metrics
		ProxyRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemProxy,
			Name:      metricsNameRequestsTotal,
			Help:      "Total number of requests made through proxies",
		}, []string{metricsLabelProxy}),
		ProxyFailures: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemProxy,
			Name:      "failures_total",
			Help:      "Total number of proxy failures",
		}, []string{metricsLabelProxy}),
		ProxiesAvailable: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemProxy,
			Name:      "available",
			Help:      "Number of currently available proxies",
		}),

		// Downloader metrics
		DownloaderRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemDownloader,
			Name:      metricsNameRequestsTotal,
			Help:      "Total number of download requests",
		}, []string{metricsSubsystemDownloader, "status"}),
		DownloaderErrors: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemDownloader,
			Name:      "errors_total",
			Help:      "Total number of download errors",
		}, []string{metricsSubsystemDownloader, "error_type"}),

		// Dependency manager metrics
		DependencyBinaryDownloadsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemDepManager,
			Name:      "binary_downloads_total",
			Help:      "Total number of successful dependency binary downloads and installs",
		}, []string{"binary", "reason"}),
	}

	return metrics
}

// Handler returns the Prometheus HTTP handler bound to this metrics registry.
func (m *Metrics) Handler() http.Handler {
	if m == nil || m.registry == nil {
		return promhttp.Handler()
	}

	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Registry returns metrics registry with runtime and process collectors.
func (m *Metrics) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}

	return m.registry
}

// JobTimer returns a function to record job duration.
func (m *Metrics) JobTimer() func() {
	if m == nil {
		return func() {}
	}

	start := time.Now()

	return func() {
		m.JobDuration.Observe(time.Since(start).Seconds())
	}
}

// RecordHTTPRequest records HTTP request metrics.
func (m *Metrics) RecordHTTPRequest(method string, status int, duration time.Duration, size int) {
	if m == nil {
		return
	}

	statusStr := strconv.Itoa(status)
	m.HTTPRequestsTotal.WithLabelValues(method, statusStr).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, statusStr).Observe(duration.Seconds())
	m.HTTPResponseSize.WithLabelValues(method, statusStr).Observe(float64(size))
}

// RecordJobCreated increments the jobs created counter.
func (m *Metrics) RecordJobCreated() {
	if m == nil {
		return
	}

	m.JobsCreated.Inc()
}

// RecordJobStarted increments in-progress jobs.
func (m *Metrics) RecordJobStarted() {
	if m == nil {
		return
	}

	m.JobsInProgress.Inc()
}

// RecordJobCompleted records a completed job.
func (m *Metrics) RecordJobCompleted() {
	if m == nil {
		return
	}

	m.JobsCompleted.Inc()
	m.JobsInProgress.Dec()
}

// RecordJobFailed records a failed job.
func (m *Metrics) RecordJobFailed() {
	if m == nil {
		return
	}

	m.JobsFailed.Inc()
	m.JobsInProgress.Dec()
}

// RecordJobCancelled decrements in-progress jobs for a cancelled run.
func (m *Metrics) RecordJobCancelled() {
	if m == nil {
		return
	}

	m.JobsInProgress.Dec()
}

// RecordJobDownloadedBytes increments downloaded bytes counter.
func (m *Metrics) RecordJobDownloadedBytes(size int64) {
	if m == nil || size <= 0 {
		return
	}

	m.JobDownloadBytes.Add(float64(size))
}

// RecordCleanup records cleanup metrics.
func (m *Metrics) RecordCleanup(jobs, files int) {
	if m == nil {
		return
	}

	m.CleanupJobsTotal.Add(float64(jobs))
	m.CleanupFilesTotal.Add(float64(files))
}

// RecordDownloaderRequest records a download request.
func (m *Metrics) RecordDownloaderRequest(downloader, status string) {
	if m == nil {
		return
	}

	m.DownloaderRequestsTotal.WithLabelValues(downloader, status).Inc()
}

// RecordDownloaderError records a download error.
func (m *Metrics) RecordDownloaderError(downloader, errorType string) {
	if m == nil {
		return
	}

	m.DownloaderErrors.WithLabelValues(downloader, errorType).Inc()
}

// RecordDependencyBinaryDownload records a successful dependency binary download.
func (m *Metrics) RecordDependencyBinaryDownload(binary, reason string) {
	if m == nil {
		return
	}

	m.DependencyBinaryDownloadsTotal.WithLabelValues(binary, reason).Inc()
}

// RecordProxyRequest records a proxy request.
func (m *Metrics) RecordProxyRequest(proxy string) {
	if m == nil {
		return
	}

	m.ProxyRequestsTotal.WithLabelValues(proxy).Inc()
}

// RecordProxyFailure records a proxy failure.
func (m *Metrics) RecordProxyFailure(proxy string) {
	if m == nil {
		return
	}

	m.ProxyFailures.WithLabelValues(proxy).Inc()
}

// SetProxiesAvailable sets the number of available proxies.
func (m *Metrics) SetProxiesAvailable(count int) {
	if m == nil {
		return
	}

	m.ProxiesAvailable.Set(float64(count))
}

// SetStoredJobs sets the number of stored jobs.
func (m *Metrics) SetStoredJobs(count int) {
	if m == nil {
		return
	}

	m.StoredJobsTotal.Set(float64(count))
}

// SetStoredPublications sets the number of stored publications.
func (m *Metrics) SetStoredPublications(count int) {
	if m == nil {
		return
	}

	m.StoredPublications.Set(float64(count))
}
