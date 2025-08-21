package prometheus

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Exporter provides export features to Prometheus
type Exporter struct {
	MetaReg *prometheus.Registry
	FlowReg *prometheus.Registry

	kafkaMessageCount prometheus.Counter
	kafkaOffsets      *prometheus.CounterVec
	flowBits          *prometheus.CounterVec
	flowAsPairsBytes  *prometheus.CounterVec
	flowAsPathBytes   *prometheus.CounterVec

	labels []string
}

// Initialize Prometheus Exporter
func (e *Exporter) Initialize(labels []string) {
	e.labels = labels

	// The Kafka metrics are added to the global registry.
	e.kafkaMessageCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_messages_total",
			Help: "Number of Kafka messages",
		})
	e.kafkaOffsets = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_offset_current",
			Help: "Current Kafka Offset of the consumer",
		}, []string{"topic", "partition"})
	e.MetaReg = prometheus.NewRegistry()
	e.MetaReg.MustRegister(e.kafkaMessageCount, e.kafkaOffsets)

	// Flows are stored in a separate Registry
	e.flowBits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flow_bits",
			Help: "Number of Bits received across Flows.",
		}, labels)

	e.flowAsPairsBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flow_as_hop_pair_bytes",
			Help: "Traffic volume between AS hop pairs in enriched flows",
		}, []string{"from", "to", "path_direction"})

	e.flowAsPathBytes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flow_as_path_bytes",
			Help: "Traffic volume of AS path in enriched flows",
		}, []string{"as_path"})

	e.FlowReg = prometheus.NewRegistry()
	e.FlowReg.MustRegister(e.flowBits, e.flowAsPairsBytes, e.flowAsPathBytes)

}

func (e *Exporter) ResetCounter() {
	log.Info().Msgf("Prometheus Exporter: resetting counter")
	e.kafkaOffsets.Reset()
	e.flowBits.Reset()

	e.MetaReg.Unregister(e.kafkaMessageCount)
	e.kafkaMessageCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_messages_total",
			Help: "Number of Kafka messages",
		})
	e.MetaReg.MustRegister(e.kafkaMessageCount)
}

// listen on given endpoint addr with Handler for metricPath and flowdataPath
func (e *Exporter) ServeEndpoints(segment *Prometheus) {
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
		err := http.ListenAndServe(segment.Endpoint, mux)
		if err != nil {
			log.Error().Err(err).Msgf("Prometheus Exporter: Failed to start https endpoint on port %s", segment.Endpoint)
		}
	}()
	log.Info().Msgf("Prometheus Exporter: Enabled metrics on %s and %s, listening at %s.", segment.MetricsPath, segment.FlowdataPath, segment.Endpoint)
}

func (e *Exporter) Increment(bytes uint64, packets uint64, labelset prometheus.Labels) {
	e.kafkaMessageCount.Inc()
	// e.flowNumber.With(labels).Inc()
	// flowPackets.With(labels).Add(float64(flow.GetPackets()))
	e.flowBits.With(labelset).Add(float64(bytes) * 8)
}

func (e *Exporter) IncrementCtrl(topic string, partition int32, offset int64) {
	labels := prometheus.Labels{
		"topic":     topic,
		"partition": fmt.Sprint(partition),
	}
	e.kafkaOffsets.With(labels).Add(float64(offset))
}
