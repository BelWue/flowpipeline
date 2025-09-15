// The `toptalkers_metrics` segment calculates statistics about traffic levels
// per IP address and exports them in OpenMetrics format via HTTP.
//
// Traffic is counted in bits per second and packets per second, categorized into
// forwarded and dropped traffic. By default, only the destination IP addresses
// are accounted, but the configuration allows using the source IP address or
// both addresses. For the latter, a flows number of bytes and packets are
// counted for both addresses. `connection` is used to look a specific combinations
// of "source -> target".
//
// Thresholds for bits per second or packets per second can be configured. Only
// metrics for addresses that exceeded this threshold during the last window size
// are exported. This can be used for detection of unusual or unwanted traffic
// levels. This can also be used as a flow filter: While the average traffic for
// an address is above threshold, flows are passed, other flows are dropped.
//
// The averages are calculated with a sliding window. The window size (in number
// of buckets) and the bucket duration can be configured. By default, it uses
// 60 buckets of 1 second each (1 minute of sliding window). Optionally, the
// window size for the exported metrics calculation and for the threshold check
// can be configured differently.
//
// The parameter "traffictype" is passed as OpenMetrics label, so this segment
// can be used multiple times in one pipeline without metrics getting mixed up.
package toptalkers_metrics

import (
	"sync"

	"github.com/BelWue/flowpipeline/pipeline/config/evaluation_mode"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"
)

type ToptalkersMetrics struct {
	segments.BaseFilterSegment
	PrometheusMetricsParams
	PrometheusParams
}

func (segment ToptalkersMetrics) New(config map[string]string) segments.Segment {
	newsegment := &ToptalkersMetrics{}
	newsegment.InitDefaultPrometheusParams()
	newsegment.InitDefaultPrometheusMetricParams()

	err := newsegment.ParsePrometheusConfig(config)
	if err != nil {
		log.Error().Err(err).Msg("ToptalkersMetrics: Failed parsing prometheus config")
		return nil
	}
	if config["endpoint"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'endpoint'. Using default port ':8080'")
	} else {
		newsegment.Endpoint = config["endpoint"]
	}

	if config["metricspath"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'metricspath'. Using default path 'metrics'")
	} else {
		newsegment.MetricsPath = config["metricspath"]
	}
	if config["flowdatapath"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'flowdatapath'. Using default path 'flowdata'")
	} else {
		newsegment.FlowdataPath = config["flowdatapath"]
	}
	return newsegment
}

func (segment *ToptalkersMetrics) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var promExporter = PrometheusExporter{}

	database := NewDatabase(segment.PrometheusMetricsParams, &promExporter)
	promExporter.Initialize()
	collector := NewPrometheusCollector([]*Database{&database})
	promExporter.FlowReg.MustRegister(collector)
	promExporter.ServeEndpoints(&segment.PrometheusParams)

	go database.Clock()
	go database.Cleanup()

	for msg := range segment.In {
		promExporter.KafkaMessageCount.Inc()
		var keys []string
		switch segment.EvaluationMode {
		case evaluation_mode.Source:
			keys = []string{msg.SrcAddrObj().String()}
		case evaluation_mode.Destination:
			keys = []string{msg.DstAddrObj().String()}
		case evaluation_mode.SourceAndDestination:
			keys = []string{msg.SrcAddrObj().String(), msg.DstAddrObj().String()}
		}
		forward := false
		for _, key := range keys {
			record := database.GetRecord(key, msg.SrcAddrObj().String(), msg.DstAddrObj().String())
			record.Append(msg)
			if record.AboveThreshold().Load() {
				forward = true
			}
		}
		if forward {
			segment.Out <- msg
		} else if segment.Drops != nil {
			segment.Drops <- msg
		}
	}
}

func init() {
	segment := &ToptalkersMetrics{}
	segments.RegisterSegment("toptalkers_metrics", segment)
}
