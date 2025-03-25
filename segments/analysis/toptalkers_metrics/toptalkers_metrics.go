package flowpipeline

import (
	"bufio"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const cleanupWindowSizes = 5

type Record struct {
	FwdBytes       []uint64
	FwdPackets     []uint64
	DropBytes      []uint64
	DropPackets    []uint64
	capacity       int
	pointer        int
	aboveThreshold atomic.Bool
	sync.RWMutex
}

type ToptalkersMetrics struct {
	segments.BaseFilterSegment
	writer   *bufio.Writer
	database *Database

	TrafficType      string // optional, default is "", name for the traffic type (included as label)
	Buckets          int    // optional, default is 60, sets the number of seconds used as a sliding window size
	ThresholdBuckets int    // optional, use the last N buckets for calculation of averages, default: $Buckets
	ReportBuckets    int    // optional, use the last N buckets to calculate averages that are reported as result, default: $Buckets
	BucketDuration   int    // optional, duration of a bucket, default is 1 second
	ThresholdBps     uint64 // optional, default is 0, only log talkers with an average bits per second rate higher than this value
	ThresholdPps     uint64 // optional, default is 0, only log talkers with an average packets per second rate higher than this value
	Endpoint         string // optional, default value is ":8080"
	MetricsPath      string // optional, default is "/metrics"
	FlowdataPath     string // optional, default is "/flowdata"
	RelevantAddress  string // optional, default is "destination", options are "destination", "source", "both"
}

type Database struct {
	database         map[string]*Record
	thresholdBps     uint64
	thresholdPps     uint64
	buckets          int // seconds
	bucketDuration   int // seconds
	thresholdBuckets int
	cleanupCounter   int
	promExporter     PrometheusExporter
	stopOnce         sync.Once
	stopCleanupC     chan struct{}
	stopClockC       chan struct{}
	sync.RWMutex
}

func (db *Database) GetRecord(key string) *Record {
	db.Lock()
	defer db.Unlock()
	record, found := db.database[key]
	if !found || record == nil {
		record = NewRecord(db.buckets)
		db.database[key] = record
	}
	return record
}

func NewRecord(windowSize int) *Record {
	record := &Record{
		FwdBytes:    make([]uint64, windowSize),
		FwdPackets:  make([]uint64, windowSize),
		DropBytes:   make([]uint64, windowSize),
		DropPackets: make([]uint64, windowSize),
		capacity:    windowSize,
		pointer:     0,
	}
	return record
}

func (record *Record) Append(bytes uint64, packets uint64, statusFwd bool) {
	record.Lock()
	defer record.Unlock()
	if statusFwd == true {
		record.FwdBytes[record.pointer] += bytes
		record.FwdPackets[record.pointer] += packets
	} else {
		record.DropBytes[record.pointer] += bytes
		record.DropPackets[record.pointer] += packets
	}
}

func (record *Record) isEmpty() bool {
	record.RLock()
	defer record.RUnlock()
	for i := 0; i < record.capacity; i++ {
		if record.FwdPackets[i] > 0 || record.DropPackets[i] > 0 {
			return false
		}
	}
	return true
}

func (record *Record) GetMetrics(buckets int, bucketDuration int) (float64, float64, float64, float64) {
	// buckets == 0 means "look at the whole window"
	if buckets == 0 {
		buckets = record.capacity
	}
	sumFwdBytes := uint64(0)
	sumFwdPackets := uint64(0)
	sumDropBytes := uint64(0)
	sumDropPackets := uint64(0)
	record.RLock()
	defer record.RUnlock()
	pos := record.pointer
	for i := 0; i < buckets; i++ {
		if pos <= 0 {
			pos = record.capacity - 1
		} else {
			pos--
		}
		sumFwdBytes += record.FwdBytes[pos]
		sumFwdPackets += record.FwdPackets[pos]
		sumDropBytes += record.DropBytes[pos]
		sumDropPackets += record.DropPackets[pos]
	}
	sumFwdBps := float64(sumFwdBytes*8) / float64(buckets*bucketDuration)
	sumFwdPps := float64(sumFwdPackets) / float64(buckets*bucketDuration)
	sumDropBps := float64(sumDropBytes*8) / float64(buckets*bucketDuration)
	sumDropPps := float64(sumDropPackets) / float64(buckets*bucketDuration)
	return sumFwdBps, sumFwdPps, sumDropBps, sumDropPps
}

func (record *Record) tick(thresholdBuckets int, bucketDuration int, thresholdBps uint64, thresholdPps uint64) {
	record.Lock()
	defer record.Unlock()
	// advance pointer to the next position
	record.pointer++
	if record.pointer >= record.capacity {
		record.pointer = 0
	}
	// calculate averages and check thresholds
	if thresholdBuckets == 0 {
		// thresholdBuckets == 0 means "look at the whole window"
		thresholdBuckets = record.capacity
	}
	var sumBytes uint64
	var sumPackets uint64
	pos := record.pointer
	for i := 0; i < thresholdBuckets; i++ {
		if pos <= 0 {
			pos = record.capacity - 1
		} else {
			pos--
		}
		sumBytes = sumBytes + record.FwdBytes[pos] + record.DropBytes[pos]
		sumPackets = sumPackets + record.FwdPackets[pos] + record.DropPackets[pos]
	}
	bps := uint64(float64(sumBytes*8) / float64(bucketDuration*thresholdBuckets))
	pps := uint64(float64(sumPackets) / float64(bucketDuration*thresholdBuckets))
	if (bps > thresholdBps) && (pps > thresholdPps) {
		record.aboveThreshold.Store(true)
	} else {
		record.aboveThreshold.Store(false)
	}
	// clear the current bucket
	record.FwdBytes[record.pointer] = 0
	record.FwdPackets[record.pointer] = 0
	record.DropBytes[record.pointer] = 0
	record.DropPackets[record.pointer] = 0
}

func (db *Database) clock() {
	ticker := time.NewTicker(time.Duration(db.bucketDuration) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			db.Lock()
			for _, record := range db.database {
				record.tick(db.thresholdBuckets, db.bucketDuration, db.thresholdBps, db.thresholdPps)
			}
			db.Unlock()
		case <-db.stopClockC:
			return
		}
	}
}

func (db *Database) cleanup() {
	ticker := time.NewTicker(time.Duration(db.bucketDuration*db.buckets) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			db.Lock()
			db.cleanupCounter--
			if db.cleanupCounter <= 0 {
				db.cleanupCounter = db.buckets * cleanupWindowSizes
				for key, record := range db.database {
					if record.isEmpty() == true {
						delete(db.database, key)
					}
				}
			}
			db.promExporter.dbSize.Set(float64(len(db.database)))
			db.Unlock()
		case <-db.stopCleanupC:
			return
		}
	}
}

func (db *Database) GetAllRecords() <-chan struct {
	key    string
	record *Record
} {
	out := make(chan struct {
		key    string
		record *Record
	})
	go func() {
		db.Lock()
		defer func() {
			db.Unlock()
			close(out)
		}()
		for key, record := range db.database {
			out <- struct {
				key    string
				record *Record
			}{key, record}
		}
	}()
	return out
}

// Exporter provides export features to Prometheus
type PrometheusExporter struct {
	MetaReg *prometheus.Registry
	FlowReg *prometheus.Registry

	kafkaMessageCount prometheus.Counter
	dbSize            prometheus.Gauge
}

// Initialize Prometheus Exporter
func (e *PrometheusExporter) Initialize(collector *PrometheusCollector) {
	// The Kafka metrics are added to the global registry.
	e.kafkaMessageCount = prometheus.NewCounter(
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
	e.MetaReg.MustRegister(e.kafkaMessageCount)
	e.MetaReg.MustRegister(e.dbSize)

	e.FlowReg = prometheus.NewRegistry()
	e.FlowReg.MustRegister(collector)
}

// listen on given endpoint addr with Handler for metricPath and flowdataPath
func (e *PrometheusExporter) ServeEndpoints(segment *ToptalkersMetrics) {
	mux := http.NewServeMux()
	mux.Handle(segment.MetricsPath, promhttp.HandlerFor(e.MetaReg, promhttp.HandlerOpts{}))
	mux.Handle(segment.FlowdataPath, promhttp.HandlerFor(e.FlowReg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Flow Exporter</title></head>
			<body>
			<h1>Flow Exporter</h1>
			<p><a href="` + segment.MetricsPath + `">Metrics</p>
			<p><a href="` + segment.FlowdataPath + `">Flow Data</p>
			</body>
		</html>`))
	})
	go func() {
		http.ListenAndServe(segment.Endpoint, mux)
	}()
	log.Info().Msgf("Enabled metrics on %s and %s, listening at %s.", segment.MetricsPath, segment.FlowdataPath, segment.Endpoint)
}

func (segment ToptalkersMetrics) New(config map[string]string) segments.Segment {
	newsegment := &ToptalkersMetrics{
		Buckets:          60,
		ThresholdBuckets: 60,
		ReportBuckets:    60,
		BucketDuration:   1,
		TrafficType:      "",
		Endpoint:         ":8080",
		MetricsPath:      "/metrics",
		FlowdataPath:     "/flowdata",
		RelevantAddress:  "destination",
	}

	if config["buckets"] != "" {
		if parsedBuckets, err := strconv.ParseInt(config["buckets"], 10, 64); err == nil {
			newsegment.Buckets = int(parsedBuckets)
			if newsegment.Buckets <= 0 {
				log.Error().Msg(": Buckets has to be >0.")
				return nil
			}
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'buckets' parameter, using default 60.")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'buckets' set to default 60.")
	}

	if config["thresholdbuckets"] != "" {
		if parsedThresholdBuckets, err := strconv.ParseInt(config["thresholdbuckets"], 10, 64); err == nil {
			newsegment.ThresholdBuckets = int(parsedThresholdBuckets)
			if newsegment.ThresholdBuckets <= 0 {
				log.Error().Msg(": thresholdbuckets has to be >0.")
				return nil
			}
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'thresholdbuckets' parameter, using default (60 buckets).")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'thresholdbuckets' set to default (60 buckets).")
	}

	if config["reportbuckets"] != "" {
		if parsedReportBuckets, err := strconv.ParseInt(config["reportbuckets"], 10, 64); err == nil {
			newsegment.ReportBuckets = int(parsedReportBuckets)
			if newsegment.ReportBuckets <= 0 {
				log.Error().Msg(": reportbuckets has to be >0.")
				return nil
			}
		} else {
			log.Error().Msg("ReportPrometheus: Could not parse 'reportbuckets' parameter, using default (60 buckets).")
		}
	} else {
		log.Info().Msg("ReportPrometheus: 'reportbuckets' set to default (60 buckets).")
	}

	if config["traffictype"] != "" {
		newsegment.TrafficType = config["traffictype"]
	} else {
		log.Info().Msg("ToptalkersMetrics: 'traffictype' is empty.")
	}

	if config["thresholdbps"] != "" {
		if parsedThresholdBps, err := strconv.ParseUint(config["thresholdbps"], 10, 32); err == nil {
			newsegment.ThresholdBps = parsedThresholdBps
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'thresholdbps' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'thresholdbps' set to default '0'.")
	}

	if config["thresholdpps"] != "" {
		if parsedThresholdPps, err := strconv.ParseUint(config["thresholdpps"], 10, 32); err == nil {
			newsegment.ThresholdPps = parsedThresholdPps
		} else {
			log.Error().Msg("ToptalkersMetrics: Could not parse 'thresholdpps' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("ToptalkersMetrics: 'thresholdpps' set to default '0'.")
	}

	if config["endpoint"] == "" {
		log.Info().Msg("ToptalkersMetrics Missing configuration parameter 'endpoint'. Using default port ':8080'")
	} else {
		newsegment.Endpoint = config["endpoint"]
	}

	if config["metricspath"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'metricspath'. Using default path 'metrics'")
	} else {
		newsegment.FlowdataPath = config["metricspath"]
	}
	if config["flowdatapath"] == "" {
		log.Info().Msg("ThresholdToptalkersMetrics: Missing configuration parameter 'flowdatapath'. Using default path 'flowdata'")
	} else {
		newsegment.FlowdataPath = config["flowdatapath"]
	}

	switch config["relevantaddress"] {
	case
		"destination",
		"source",
		"both":
		newsegment.RelevantAddress = config["relevantaddress"]
	case "":
		log.Info().Msg("ToptalkersMetrics: 'relevantaddress' set to default 'destination'.")
	default:
		log.Error().Msg("ToptalkersMetrics: Could not parse 'relevantaddress', using default value 'destination'.")
	}
	return newsegment
}

func (segment *ToptalkersMetrics) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var promExporter = PrometheusExporter{}
	collector := &PrometheusCollector{segment}
	promExporter.Initialize(collector)
	promExporter.ServeEndpoints(segment)

	segment.database = &Database{
		database:         map[string]*Record{},
		thresholdBps:     segment.ThresholdBps,
		thresholdPps:     segment.ThresholdPps,
		thresholdBuckets: segment.ThresholdBuckets,
		cleanupCounter:   segment.Buckets * cleanupWindowSizes, // cleanup every N windows
		promExporter:     promExporter,
		buckets:          segment.Buckets,
		bucketDuration:   segment.BucketDuration,
		stopCleanupC:     make(chan struct{}),
		stopClockC:       make(chan struct{}),
	}
	go segment.database.clock()
	go segment.database.cleanup()

	for msg := range segment.In {
		promExporter.kafkaMessageCount.Inc()
		var keys []string
		if segment.RelevantAddress == "source" {
			keys = []string{msg.SrcAddrObj().String()}
		} else if segment.RelevantAddress == "destination" {
			keys = []string{msg.DstAddrObj().String()}
		} else if segment.RelevantAddress == "both" {
			keys = []string{msg.SrcAddrObj().String(), msg.DstAddrObj().String()}
		}
		forward := false
		for _, key := range keys {
			record := segment.database.GetRecord(key)
			record.Append(msg.Bytes, msg.Packets, msg.IsForwarded())
			if record.aboveThreshold.Load() == true {
				forward = true
			}
		}
		if forward == true {
			segment.Out <- msg
		} else if segment.Drops != nil {
			segment.Drops <- msg
		}
	}
}

var (
	trafficBpsDesc = prometheus.NewDesc(
		"traffic_bps",
		"Traffic volume in bits per second for a given address.",
		[]string{"traffic_type", "address", "forwarding_status"}, nil,
	)
	trafficPpsDesc = prometheus.NewDesc(
		"traffic_pps",
		"Traffic in packets per second for a given address.",
		[]string{"traffic_type", "address", "forwarding_status"}, nil,
	)
)

type PrometheusCollector struct {
	segment *ToptalkersMetrics
}

func (collector *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- trafficBpsDesc
	ch <- trafficPpsDesc
}
func (collector *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	for entry := range collector.segment.database.GetAllRecords() {
		key := entry.key
		record := entry.record
		// check if thresholds are exceeded
		buckets := collector.segment.ReportBuckets
		bucketDuration := collector.segment.BucketDuration
		if record.aboveThreshold.Load() == true {
			sumFwdBps, sumFwdPps, sumDropBps, sumDropPps := record.GetMetrics(buckets, bucketDuration)
			ch <- prometheus.MustNewConstMetric(
				trafficBpsDesc,
				prometheus.GaugeValue,
				sumFwdBps,
				collector.segment.TrafficType, key, "forwarded",
			)
			ch <- prometheus.MustNewConstMetric(
				trafficBpsDesc,
				prometheus.GaugeValue,
				sumDropBps,
				collector.segment.TrafficType, key, "dropped",
			)
			ch <- prometheus.MustNewConstMetric(
				trafficPpsDesc,
				prometheus.GaugeValue,
				sumFwdPps,
				collector.segment.TrafficType, key, "forwarded",
			)
			ch <- prometheus.MustNewConstMetric(
				trafficPpsDesc,
				prometheus.GaugeValue,
				sumDropPps,
				collector.segment.TrafficType, key, "dropped",
			)
		}
	}
}

func init() {
	segment := &ToptalkersMetrics{}
	segments.RegisterSegment("toptalkers_metrics", segment)
}
