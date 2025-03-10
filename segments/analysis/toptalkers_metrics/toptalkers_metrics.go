package toptalkers_metrics

import (
	"log"
	"sync"

	"github.com/BelWue/flowpipeline/segments"
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
		log.Println(err.Error())
		return nil
	}
	if config["endpoint"] == "" {
		log.Println("[info] ToptalkersMetrics Missing configuration parameter 'endpoint'. Using default port \":8080\"")
	} else {
		newsegment.Endpoint = config["endpoint"]
	}

	if config["metricspath"] == "" {
		log.Println("[info] ToptalkersMetrics: Missing configuration parameter 'metricspath'. Using default path \"/metrics\"")
	} else {
		newsegment.MetricsPath = config["metricspath"]
	}
	if config["flowdatapath"] == "" {
		log.Println("[info] ThresholdToptalkersMetrics: Missing configuration parameter 'flowdatapath'. Using default path \"/flowdata\"")
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
	collector := NewPrometheusCollector([]*Database{&database})
	promExporter.Initialize()
	promExporter.FlowReg.MustRegister(collector)
	promExporter.ServeEndpoints(&segment.PrometheusParams)

	go database.Clock()
	go database.Cleanup()

	for msg := range segment.In {
		promExporter.KafkaMessageCount.Inc()
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
			record := database.GetRecord(key)
			record.Append(msg.Bytes, msg.Packets, msg.IsForwarded())
			if record.aboveThreshold.Load() {
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
