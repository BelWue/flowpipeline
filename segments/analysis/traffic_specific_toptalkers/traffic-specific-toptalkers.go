// This segment is used to alert on flows reaching a specified threshold
package traffic_specific_toptalkers

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowfilter/parser"
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/pipeline/config"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/analysis/toptalkers_metrics"
	"github.com/BelWue/flowpipeline/segments/filter/flowfilter"
)

type TrafficSpecificToptalkers struct {
	segments.BaseSegment
	toptalkers_metrics.PrometheusParams
	ThresholdMetricDefinition []*ThresholdMetric
	RelevantAddress           string // optional, default is "destination", options are "destination", "source", "both", "connection"
}

type ThresholdMetric struct {
	toptalkers_metrics.PrometheusMetricsParams

	Database         *toptalkers_metrics.Database
	SubDefinitions   []*ThresholdMetric
	Expression       *parser.Expression
	FilterDefinition string
}

func (segment TrafficSpecificToptalkers) New(config map[string]string) segments.Segment {
	newSegment := &TrafficSpecificToptalkers{}
	newSegment.InitDefaultPrometheusParams()
	if config["endpoint"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'endpoint'. Using default port \":8080\"")
	} else {
		newSegment.Endpoint = config["endpoint"]
	}

	if config["metricspath"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'metricspath'. Using default path \"/metrics\"")
	} else {
		newSegment.FlowdataPath = config["metricspath"]
	}
	if config["flowdatapath"] == "" {
		log.Info().Msg("ThresholdToptalkersMetrics: Missing configuration parameter 'flowdatapath'. Using default path \"/flowdata\"")
	} else {
		newSegment.FlowdataPath = config["flowdatapath"]
	}
	if config["relevantaddress"] != "" {
		newSegment.RelevantAddress = config["relevantaddress"]
	} else {
		newSegment.RelevantAddress = ""
	}

	return newSegment
}

func (segment *TrafficSpecificToptalkers) AddCustomConfig(config config.Config) {
	for _, definition := range config.ThresholdMetricDefinition {
		metric, err := segment.metricFromDefinition(definition)
		if err != nil {
			log.Error().Err(err).Msg("ThresholdToptalkersMetrics: Failed to add custom config")
		}
		segment.ThresholdMetricDefinition = append(segment.ThresholdMetricDefinition, metric)
	}
}

func (segment *TrafficSpecificToptalkers) metricFromDefinition(definition *config.ThresholdMetricDefinition) (*ThresholdMetric, error) {
	var err error
	metric := ThresholdMetric{}
	metric.PrometheusMetricsParamsDefinition = definition.PrometheusMetricsParamsDefinition
	metric.FilterDefinition = definition.FilterDefinition
	metric.InitDefaultPrometheusMetricParams()

	if segment.RelevantAddress != "" {
		metric.RelevantAddress = segment.RelevantAddress
	}

	metric.Expression, err = parser.Parse(definition.FilterDefinition)
	if err != nil {
		log.Error().Err(err).Msgf("ThresholdToptalkersMetrics: Syntax error in filter expression\"%s\"", definition.FilterDefinition)
		return nil, err
	}

	for _, subDefinition := range definition.SubDefinitions {
		subMetric, err := segment.metricFromDefinition(subDefinition)
		if err != nil {
			return nil, err
		}
		metric.SubDefinitions = append(metric.SubDefinitions, subMetric)
	}

	return &metric, nil
}

func (segment *TrafficSpecificToptalkers) Run(wg *sync.WaitGroup) {
	var allDatabases *[]*toptalkers_metrics.Database
	defer func() {
		close(segment.Out)
		for _, db := range *allDatabases {
			db.StopTimers()
		}
		wg.Done()
	}()

	var promExporter = toptalkers_metrics.PrometheusExporter{}
	promExporter.Initialize()

	allDatabases = initDatabasesAndCollector(promExporter, segment)

	//start timers
	promExporter.ServeEndpoints(&segment.PrometheusParams)
	for _, db := range *allDatabases {
		go db.Clock()
		go db.Cleanup()
	}

	filter := &flowfilter.Filter{}
	log.Info().Msgf("Threshold Metric Report runing on %s", segment.Endpoint)
	for msg := range segment.In {
		promExporter.KafkaMessageCount.Inc()
		for _, filterDef := range segment.ThresholdMetricDefinition {
			addMessageToMatchingToptalkers(msg, filterDef, filter)
		}
		segment.Out <- msg
	}
}

func initDatabasesAndCollector(promExporter toptalkers_metrics.PrometheusExporter, segment *TrafficSpecificToptalkers) *[]*toptalkers_metrics.Database {
	allDatabases := []*toptalkers_metrics.Database{}
	for _, filterDef := range segment.ThresholdMetricDefinition {
		databases := initDatabasesForFilter(filterDef, &promExporter)
		allDatabases = append(allDatabases, databases...)
	}

	collector := toptalkers_metrics.NewPrometheusCollector(allDatabases)
	promExporter.FlowReg.MustRegister(collector)
	return &allDatabases
}

func initDatabasesForFilter(filterDef *ThresholdMetric, promExporter *toptalkers_metrics.PrometheusExporter) []*toptalkers_metrics.Database {
	databases := []*toptalkers_metrics.Database{}
	if filterDef.PrometheusMetricsParams.TrafficType != "" { //defined a metric that should be in prometheus
		database := toptalkers_metrics.NewDatabase(filterDef.PrometheusMetricsParams, promExporter)

		filterDef.Database = &database
		databases = append(databases, &database)
	}
	for _, subDef := range filterDef.SubDefinitions {
		databases = append(databases, initDatabasesForFilter(subDef, promExporter)...)
	}
	return databases
}

func addMessageToMatchingToptalkers(msg *pb.EnrichedFlow, definition *ThresholdMetric, filter *flowfilter.Filter) {
	if match, _ := filter.CheckFlow(definition.Expression, msg); match {
		// Update Counters if definition has a prometheus label defined
		if definition.PrometheusMetricsParams.TrafficType != "" {
			var keys []string
			switch definition.PrometheusMetricsParams.RelevantAddress {
			case "source":
				keys = []string{msg.SrcAddrObj().String()}
			case "destination":
				keys = []string{msg.DstAddrObj().String()}
			case "both":
				keys = []string{msg.SrcAddrObj().String(), msg.DstAddrObj().String()}
			case "connection":
				keys = []string{fmt.Sprintf("%s -> %s", msg.SrcAddrObj().String(), msg.DstAddrObj().String())}
			}
			for _, key := range keys {
				record := definition.Database.GetTypedRecord(definition.PrometheusMetricsParams.TrafficType, key)
				record.Append(msg.Bytes, msg.Packets, msg.IsForwarded())
			}
		}

		//also check subfilters
		for _, subdefinition := range definition.SubDefinitions {
			addMessageToMatchingToptalkers(msg, subdefinition, filter)
		}
	}
}

func init() {
	segment := &TrafficSpecificToptalkers{}
	segments.RegisterSegment("traffic_specific_toptalkers", segment)
}
