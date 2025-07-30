package toptalkers_metrics

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/BelWue/flowpipeline/pipeline/config"
	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusCollector struct {
	Databases      []*Database
	trafficBpsDesc *prometheus.Desc
	trafficPpsDesc *prometheus.Desc
}

type PrometheusParams struct {
	Endpoint     string // optional, default value is ":8080"
	MetricsPath  string // optional, default is "/metrics"
	FlowdataPath string // optional, default is "/flowdata"
}

type PrometheusMetricsParams struct {
	config.PrometheusMetricsParamsDefinition
	CleanupWindowSizes int
}

func NewPrometheusCollector(databases []*Database) *PrometheusCollector {
	coll := PrometheusCollector{
		Databases: databases,
	}
	coll.trafficBpsDesc = prometheus.NewDesc(
		"traffic_bps",
		"Traffic volume in bits per second, for a given address",
		[]string{"traffic_type", "address", "forwarding_status"}, nil,
	)
	coll.trafficPpsDesc = prometheus.NewDesc(
		"traffic_pps",
		"Traffic in packets per second, for a given address.",
		[]string{"traffic_type", "address", "forwarding_status"}, nil,
	)
	return &coll
}

func (params *PrometheusParams) InitDefaultPrometheusParams() {
	params.Endpoint = ":8080"
	params.MetricsPath = "/metrics"
	params.FlowdataPath = "/flowdata"
}

func (params *PrometheusMetricsParams) InitDefaultPrometheusMetricParams() {
	if params.Buckets == 0 {
		params.Buckets = 60
	}
	if params.ThresholdBuckets == 0 {
		params.ThresholdBuckets = 60
	}
	if params.ReportBuckets == 0 {
		params.ReportBuckets = 60
	}
	if params.BucketDuration == 0 {
		params.BucketDuration = 1
	}
	if params.RelevantAddress == "" {
		params.RelevantAddress = "destination"
	}
	if params.CleanupWindowSizes == 0 {
		params.CleanupWindowSizes = 5
	}
}

func (prometheusParams *PrometheusMetricsParams) ParsePrometheusConfig(config map[string]string) error {
	if config["buckets"] != "" {
		if parsedBuckets, err := strconv.ParseInt(config["buckets"], 10, 64); err == nil {
			if parsedBuckets <= 0 {
				return errors.New("ToptalkersMetrics: Buckets has to be >0")
			}
			if parsedBuckets > math.MaxInt {
				return errors.New("ToptalkersMetrics: Buckets out of range")
			}
			prometheusParams.Buckets = int(parsedBuckets)
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'buckets' parameter, using default 60.")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'buckets' set to default 60.")
	}

	if config["thresholdbuckets"] != "" {
		if parsedThresholdBuckets, err := strconv.ParseInt(config["thresholdbuckets"], 10, 64); err == nil {

			if parsedThresholdBuckets <= 0 {
				return errors.New("ToptalkersMetrics: Thresholdbuckets has to be >0")
			}
			if parsedThresholdBuckets > math.MaxInt {
				return errors.New("ToptalkersMetrics: Thresholdbuckets out of range")
			}
			prometheusParams.ThresholdBuckets = int(parsedThresholdBuckets)
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'thresholdbuckets' parameter, using default (60 buckets).")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'thresholdbuckets' set to default (60 buckets).")
	}

	if config["reportbuckets"] != "" {
		if parsedReportBuckets, err := strconv.ParseInt(config["reportbuckets"], 10, 64); err == nil {
			if parsedReportBuckets <= 0 {
				return errors.New("ToptalkersMetrics: Reportbuckets has to be >0")
			}
			if parsedReportBuckets > math.MaxInt {
				return errors.New("ToptalkersMetrics: Reportbuckets out of range")
			}
			prometheusParams.ReportBuckets = int(parsedReportBuckets)
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'reportbuckets' parameter, using default (60 buckets).")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'reportbuckets' set to default (60 buckets).")
	}

	if config["traffictype"] != "" {
		prometheusParams.TrafficType = config["traffictype"]
	} else {
		log.Info().Msg("ToptalkersMetrics: 'traffictype' is empty.")
	}

	if config["thresholdbps"] != "" {
		if parsedThresholdBps, err := strconv.ParseUint(config["thresholdbps"], 10, 32); err == nil {
			prometheusParams.ThresholdBps = parsedThresholdBps
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'thresholdbps' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'thresholdbps' set to default '0'.")
	}

	if config["thresholdpps"] != "" {
		if parsedThresholdPps, err := strconv.ParseUint(config["thresholdpps"], 10, 32); err == nil {
			prometheusParams.ThresholdPps = parsedThresholdPps
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'thresholdpps' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'thresholdpps' set to default '0'.")
	}

	switch config["relevantaddress"] {
	case
		"destination",
		"source",
		"both",
		"connection":
		prometheusParams.RelevantAddress = config["relevantaddress"]
	case "":
		log.Info().Msg("ToptalkersMetrics: 'relevantaddress' set to default 'destination'.")
	default:
		log.Error().Msg("ToptalkersMetrics: Could not parse 'relevantaddress', using default value 'destination'.")
	}
	return nil
}

func (c *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.trafficBpsDesc
	ch <- c.trafficPpsDesc
}
func (collector *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	for _, db := range collector.Databases {
		for entry := range db.GetAllRecords() {
			record := entry.record
			// check if thresholds are exceeded
			buckets := db.ReportBuckets
			bucketDuration := db.BucketDuration
			if record.aboveThreshold.Load() {
				sumFwdBps, sumFwdPps, sumDropBps, sumDropPps, address := record.GetMetrics(buckets, bucketDuration)
				ch <- prometheus.MustNewConstMetric(
					collector.trafficBpsDesc,
					prometheus.GaugeValue,
					sumFwdBps,
					db.TrafficType, address, "forwarded",
				)
				ch <- prometheus.MustNewConstMetric(
					collector.trafficBpsDesc,
					prometheus.GaugeValue,
					sumDropBps,
					db.TrafficType, address, "dropped",
				)
				ch <- prometheus.MustNewConstMetric(
					collector.trafficPpsDesc,
					prometheus.GaugeValue,
					sumFwdPps,
					db.TrafficType, address, "forwarded",
				)
				ch <- prometheus.MustNewConstMetric(
					collector.trafficPpsDesc,
					prometheus.GaugeValue,
					sumDropPps,
					db.TrafficType, address, "dropped",
				)
			}
		}
	}
}

// Exporter provides export features to Prometheus
type PrometheusExporter struct {
	MetaReg *prometheus.Registry
	FlowReg *prometheus.Registry

	KafkaMessageCount prometheus.Counter
	dbSize            prometheus.Gauge
}

// Initialize Prometheus Exporter
func (e *PrometheusExporter) Initialize() {
	e.KafkaMessageCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_messages_total",
			Help: "Number of Kafka messages",
		})
	e.dbSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "toptalkers_db_size",
			Help: "Number of Keys in the current toptalkers database",
		})
	e.MetaReg = prometheus.NewRegistry()
	e.FlowReg = prometheus.NewRegistry()
	e.MetaReg.MustRegister(e.KafkaMessageCount)
	e.MetaReg.MustRegister(e.dbSize)
}

// listen on given endpoint addr with Handler for metricPath and flowdataPath
func (e *PrometheusExporter) ServeEndpoints(promParams *PrometheusParams) {
	mux := http.NewServeMux()
	mux.Handle(promParams.MetricsPath, promhttp.HandlerFor(e.MetaReg, promhttp.HandlerOpts{}))
	mux.Handle(promParams.FlowdataPath, promhttp.HandlerFor(e.FlowReg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Flow Exporter</title></head>
			<body>
			<h1>Flow Exporter</h1>
			<p><a href="` + promParams.MetricsPath + `">Metrics</p>
			<p><a href="` + promParams.FlowdataPath + `">Flow Data</p>
			</body>
		</html>`))
	})
	go func() {
		err := http.ListenAndServe(promParams.Endpoint, mux)
		if err != nil {
			log.Error().Err(err).Msgf("ToptalkersMetrics: Failed to start https endpoint on port %s", promParams.Endpoint)
		}
	}()
	log.Info().Msgf("ToptalkersMetrics: Enabled metrics on %s and %s, listening at %s.", promParams.MetricsPath, promParams.FlowdataPath, promParams.Endpoint)
}
