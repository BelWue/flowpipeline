package toptalkers_metrics

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusCollector struct {
	database       *Database
	ReportBuckets  int
	BucketDuration int
	TrafficType    string
	trafficBpsDesc *prometheus.Desc
	trafficPpsDesc *prometheus.Desc
}

type PrometheusMetricsParams struct {
	TrafficType      string `yaml:"traffictype,omitempty"`      // optional, default is "", name for the traffic type (included as label)
	Buckets          int    `yaml:"buckets,omitempty"`          // optional, default is 60, sets the number of seconds used as a sliding window size
	ThresholdBuckets int    `yaml:"thresholdbuckets,omitempty"` // optional, use the last N buckets for calculation of averages, default: $Buckets
	ReportBuckets    int    `yaml:"reportbuckets,omitempty"`    // optional, use the last N buckets to calculate averages that are reported as result, default: $Buckets
	BucketDuration   int    `yaml:"bucketduration,omitempty"`   // optional, duration of a bucket, default is 1 second
	ThresholdBps     uint64 `yaml:"thresholdbps,omitempty"`     // optional, default is 0, only log talkers with an average bits per second rate higher than this value
	ThresholdPps     uint64 `yaml:"thresholdpps,omitempty"`     // optional, default is 0, only log talkers with an average packets per second rate higher than this value
	RelevantAddress  string `yaml:"relevantaddress,omitempty"`  // optional, default is "destination", options are "destination", "source", "both"
}

type PrometheusParams struct {
	Endpoint     string // optional, default value is ":8080"
	MetricsPath  string // optional, default is "/metrics"
	FlowdataPath string // optional, default is "/flowdata"
}

func NewPrometheusCollector(database *Database, trafficType string, reportBuckets int) *PrometheusCollector {
	coll := PrometheusCollector{
		database:       database,
		ReportBuckets:  reportBuckets,
		BucketDuration: database.bucketDuration,
		TrafficType:    trafficType,
	}
	coll.InitDescriptors()
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
}

func (prometheusParams *PrometheusMetricsParams) ParsePrometheusConfig(config map[string]string) error {
	if config["buckets"] != "" {
		if parsedBuckets, err := strconv.ParseInt(config["buckets"], 10, 64); err == nil {
			prometheusParams.Buckets = int(parsedBuckets)
			if prometheusParams.Buckets <= 0 {
				return errors.New("[error] : Buckets has to be >0")
			}
		} else {
			log.Println("[error] ToptalkersMetrics: Could not parse 'buckets' parameter, using default 60.")
		}
	} else {
		log.Println("[info] ToptalkersMetrics: 'buckets' set to default 60.")
	}

	if config["thresholdbuckets"] != "" {
		if parsedThresholdBuckets, err := strconv.ParseInt(config["thresholdbuckets"], 10, 64); err == nil {
			prometheusParams.ThresholdBuckets = int(parsedThresholdBuckets)
			if prometheusParams.ThresholdBuckets <= 0 {
				return errors.New("[error] : thresholdbuckets has to be >0")
			}
		} else {
			log.Println("[error] ToptalkersMetrics: Could not parse 'thresholdbuckets' parameter, using default (60 buckets).")
		}
	} else {
		log.Println("[info] ToptalkersMetrics: 'thresholdbuckets' set to default (60 buckets).")
	}

	if config["reportbuckets"] != "" {
		if parsedReportBuckets, err := strconv.ParseInt(config["reportbuckets"], 10, 64); err == nil {
			prometheusParams.ReportBuckets = int(parsedReportBuckets)
			if prometheusParams.ReportBuckets <= 0 {
				return errors.New("[error] : reportbuckets has to be >0")
			}
		} else {
			log.Println("[error] ReportPrometheus: Could not parse 'reportbuckets' parameter, using default (60 buckets).")
		}
	} else {
		log.Println("[info] ReportPrometheus: 'reportbuckets' set to default (60 buckets).")
	}

	if config["traffictype"] != "" {
		prometheusParams.TrafficType = config["traffictype"]
	} else {
		log.Println("[info] ToptalkersMetrics: 'traffictype' is empty.")
	}

	if config["thresholdbps"] != "" {
		if parsedThresholdBps, err := strconv.ParseUint(config["thresholdbps"], 10, 32); err == nil {
			prometheusParams.ThresholdBps = parsedThresholdBps
		} else {
			log.Println("[error] ToptalkersMetrics: Could not parse 'thresholdbps' parameter, using default 0.")
		}
	} else {
		log.Println("[info] ToptalkersMetrics: 'thresholdbps' set to default '0'.")
	}

	if config["thresholdpps"] != "" {
		if parsedThresholdPps, err := strconv.ParseUint(config["thresholdpps"], 10, 32); err == nil {
			prometheusParams.ThresholdPps = parsedThresholdPps
		} else {
			log.Println("[error] ToptalkersMetrics: Could not parse 'thresholdpps' parameter, using default 0.")
		}
	} else {
		log.Println("[info] ToptalkersMetrics: 'thresholdpps' set to default '0'.")
	}

	switch config["relevantaddress"] {
	case
		"destination",
		"source",
		"both":
		prometheusParams.RelevantAddress = config["relevantaddress"]
	case "":
		log.Println("[info] ToptalkersMetrics: 'relevantaddress' set to default 'destination'.")
	default:
		log.Println("[error] ToptalkersMetrics: Could not parse 'relevantaddress', using default value 'destination'.")
	}
	return nil
}

func (c *PrometheusCollector) InitDescriptors() {
	var (
		bps_fqName string
		pps_fqName string
		bps_help   string
		pps_help   string
	)

	if c.TrafficType != "" {
		bps_fqName = c.TrafficType + "_traffic_bps"
		pps_fqName = c.TrafficType + "_traffic_pps"
		bps_help = "Traffic volume in bits per second of traffic type " + c.TrafficType + ", for a given address"
		pps_help = "Traffic in packets per second of traffic type " + c.TrafficType + ", for a given address."
	} else {
		bps_fqName = "traffic_bps"
		pps_fqName = "traffic_pps"
		bps_help = "Traffic volume in bits per second, for a given address"
		pps_help = "Traffic in packets per second, for a given address."
	}

	c.trafficBpsDesc = prometheus.NewDesc(
		bps_fqName,
		bps_help,
		[]string{"traffic_type", "address", "forwarding_status"}, nil,
	)
	c.trafficPpsDesc = prometheus.NewDesc(
		pps_fqName,
		pps_help,
		[]string{"traffic_type", "address", "forwarding_status"}, nil,
	)
}

func (c *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.trafficBpsDesc
	ch <- c.trafficPpsDesc
}
func (collector *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	for entry := range collector.database.GetAllRecords() {
		key := entry.key
		record := entry.record
		// check if thresholds are exceeded
		buckets := collector.ReportBuckets
		bucketDuration := collector.BucketDuration
		if record.aboveThreshold.Load() {
			sumFwdBps, sumFwdPps, sumDropBps, sumDropPps := record.GetMetrics(buckets, bucketDuration)
			ch <- prometheus.MustNewConstMetric(
				collector.trafficBpsDesc,
				prometheus.GaugeValue,
				sumFwdBps,
				collector.TrafficType, key, "forwarded",
			)
			ch <- prometheus.MustNewConstMetric(
				collector.trafficBpsDesc,
				prometheus.GaugeValue,
				sumDropBps,
				collector.TrafficType, key, "dropped",
			)
			ch <- prometheus.MustNewConstMetric(
				collector.trafficPpsDesc,
				prometheus.GaugeValue,
				sumFwdPps,
				collector.TrafficType, key, "forwarded",
			)
			ch <- prometheus.MustNewConstMetric(
				collector.trafficPpsDesc,
				prometheus.GaugeValue,
				sumDropPps,
				collector.TrafficType, key, "dropped",
			)
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
		http.ListenAndServe(promParams.Endpoint, mux)
	}()
	log.Printf("Enabled metrics on %s and %s, listening at %s.", promParams.MetricsPath, promParams.FlowdataPath, promParams.Endpoint)
}
