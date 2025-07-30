package monitoring

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/analysis/toptalkers_metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

type DelayMonitoring struct {
	segments.BaseSegment
	toptalkers_metrics.PrometheusParams

	SamplingRate int     // flow samplingrate fpr calculating delay - default 100
	Alpha        float64 // alpha used for the exponential window moving average?
	Endpoint     string  // optional, default value is ":8080"

	msgCounter                   int
	movingAverageProcessingDelay float64
	movingAverageTotalDelay      float64

	totalDelayPrometheusDescription      *prometheus.Desc
	processingDelayPrometheusDescription *prometheus.Desc
}

func (segment DelayMonitoring) New(config map[string]string) segments.Segment {
	newSegment := DelayMonitoring{
		msgCounter:                   0,
		movingAverageProcessingDelay: 0,
		movingAverageTotalDelay:      0,

		totalDelayPrometheusDescription: prometheus.NewDesc(
			"total_delay",
			"Exponential window moving average delay between flow end and processing time",
			[]string{}, nil,
		),
		processingDelayPrometheusDescription: prometheus.NewDesc(
			"processing_delay",
			"Exponential window moving average delay between flow received and processing time",
			[]string{}, nil,
		),
		Endpoint:     ":8080",
		SamplingRate: 1000,
		Alpha:        0.2,
	}

	if config["endpoint"] != "" {
		newSegment.Endpoint = config["endpoint"]
	}

	if config["samplingRate"] != "" {
		samplingRate, err := strconv.Atoi(config["samplingRate"])
		if err != nil {
			log.Error().Err(err).Msg("Delay Monitoring: Failed parsing parameter \"samplingRate\"")
		} else {
			newSegment.SamplingRate = samplingRate
		}
	}

	if config["alpha"] != "" {
		alpha, err := strconv.ParseFloat(config["alpha"], 64)
		if err != nil {
			log.Error().Err(err).Msg("Delay Monitoring: Failed parsing parameter \"alpha\"")
		} else {
			newSegment.Alpha = alpha
		}
	}

	return &newSegment
}

func (segment *DelayMonitoring) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	var promExporter = PrometheusExporter{}
	promExporter.Initialize()
	promExporter.DelayReg.MustRegister(segment)
	//start timers
	promExporter.ServeEndpoints(segment.Endpoint)

	log.Info().Msgf("Delay Monitoring: Prometheus running on %s", segment.Endpoint)
	for msg := range segment.In {
		segment.updateWindow(msg, &promExporter)
		segment.Out <- msg
	}
}

func (p *DelayMonitoring) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.totalDelayPrometheusDescription
	ch <- p.processingDelayPrometheusDescription
}
func (p *DelayMonitoring) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		p.totalDelayPrometheusDescription,
		prometheus.GaugeValue,
		p.movingAverageTotalDelay,
	)
	ch <- prometheus.MustNewConstMetric(
		p.processingDelayPrometheusDescription,
		prometheus.GaugeValue,
		p.movingAverageProcessingDelay,
	)
}

var lock sync.Mutex

func (segment *DelayMonitoring) updateWindow(msg *pb.EnrichedFlow, promExporter *PrometheusExporter) {
	lock.Lock()
	defer lock.Unlock()
	if segment.msgCounter >= segment.SamplingRate {
		promExporter.KafkaMessageCount.Inc()
		if msg.TimeFlowEnd == 0 {
			log.Error().Msg("Delay Monitoring: Empty field `TimeFlowEnd` for flow message - consider using segment `sync_timestamps` if your flow collector only fills TimeReceivedMs or TimeReceivedNs")
		}
		timeNow := uint64(time.Now().Unix())
		timeDiffProcessing := timeNow - msg.TimeReceived
		timeDiffTotal := timeNow - msg.TimeFlowEnd

		segment.movingAverageProcessingDelay = (segment.Alpha * float64(timeDiffProcessing)) + (1.0-segment.Alpha)*segment.movingAverageProcessingDelay
		segment.movingAverageTotalDelay = (segment.Alpha * float64(timeDiffTotal)) + (1.0-segment.Alpha)*segment.movingAverageProcessingDelay
		segment.msgCounter = 1
	} else {
		segment.msgCounter += 1
	}
}

type PrometheusExporter struct {
	MetaReg  *prometheus.Registry
	DelayReg *prometheus.Registry

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
			Name: "delay_db_size",
			Help: "Number of Keys in the current toptalkers database",
		})
	e.MetaReg = prometheus.NewRegistry()
	e.DelayReg = prometheus.NewRegistry()
	e.MetaReg.MustRegister(e.KafkaMessageCount)
	e.MetaReg.MustRegister(e.dbSize)
}

// listen on given endpoint addr with Handler for metricPath and flowdataPath
func (e *PrometheusExporter) ServeEndpoints(endpoint string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(e.MetaReg, promhttp.HandlerOpts{}))
	mux.Handle("/delay", promhttp.HandlerFor(e.DelayReg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Delay Indicator</title></head>
			<body>
			<h1>Delay Indicator</h1>
			<p><a href="/metrics">Metrics</p>
			<p><a href="/delay">Delay Measurements</p>
			</body>
		</html>`))
	})
	go func() {
		err := http.ListenAndServe(endpoint, mux)
		if err != nil {
			log.Error().Err(err).Msgf("Delay Monitoring: Failed to start https endpoint on port %s", endpoint)
		}
	}()
	log.Info().Msgf("Delay Monitoring: Enabled delay metrics on /metrics and /delay, listening at %s.", endpoint)
}

func init() {
	segment := &DelayMonitoring{}
	segments.RegisterSegment("delay_monitoring", segment)
}
