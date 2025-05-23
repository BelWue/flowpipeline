// This segment is used to alert on flows reaching a specified threshold
package traffic_specific_toptalkers

import (
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowfilter/parser"
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/analysis/toptalkers_metrics"
	"github.com/BelWue/flowpipeline/segments/filter/flowfilter"
)

type TrafficSpecificToptalkers struct {
	segments.BaseSegment
	toptalkers_metrics.PrometheusParams
	ThresholdMetricDefinition []*ThresholdMetricDefinition
}

type ThresholdMetricDefinition struct {
	toptalkers_metrics.PrometheusMetricsParams `yaml:",inline"`

	Expression       *parser.Expression
	FilterDefinition string                       `yaml:"filter,omitempty"`
	SubDefinitions   []*ThresholdMetricDefinition `yaml:"subfilter,omitempty"`
	Database         *toptalkers_metrics.Database
}

func (segment TrafficSpecificToptalkers) New(config map[string]string) segments.Segment {
	newSegment := &TrafficSpecificToptalkers{}
	newSegment.InitDefaultPrometheusParams()
	if config["endpoint"] == "" {
		log.Info().Msg("ToptalkersMetrics Missing configuration parameter 'endpoint'. Using default port \":8080\"")
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

	return newSegment
}

func (segment *TrafficSpecificToptalkers) SetThresholdMetricDefinition(definition []*ThresholdMetricDefinition) {
	segment.ThresholdMetricDefinition = definition
	for _, definition := range segment.ThresholdMetricDefinition {
		err := initThresholdMetrics(definition)
		if err != nil {
			log.Error().Err(err)
		}
	}
}

func initThresholdMetrics(definition *ThresholdMetricDefinition) error {
	definition.InitDefaultPrometheusMetricParams()
	var err error
	definition.Expression, err = parser.Parse(definition.FilterDefinition)
	if err != nil {
		log.Error().Err(err).Msg("FlowFilter: Syntax error in filter expression")
		return nil
	}
	for _, subDefinition := range definition.SubDefinitions {
		err := initThresholdMetrics(subDefinition)
		if err != nil {
			return err
		}
	}
	return nil
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

func initDatabasesForFilter(filterDef *ThresholdMetricDefinition, promExporter *toptalkers_metrics.PrometheusExporter) []*toptalkers_metrics.Database {
	databases := []*toptalkers_metrics.Database{}
	if filterDef.TrafficType != "" { //defined a metric that should be in prometheus
		database := toptalkers_metrics.NewDatabase(filterDef.PrometheusMetricsParams, promExporter)

		filterDef.Database = &database
		databases = append(databases, &database)
	}
	for _, subDef := range filterDef.SubDefinitions {
		databases = append(databases, initDatabasesForFilter(subDef, promExporter)...)
	}
	return databases
}

func addMessageToMatchingToptalkers(msg *pb.EnrichedFlow, definition *ThresholdMetricDefinition, filter *flowfilter.Filter) {
	if match, _ := filter.CheckFlow(definition.Expression, msg); match {
		// Update Counters if definition has a prometheus label defined
		if definition.TrafficType != "" {
			var keys []string
			if definition.RelevantAddress == "source" {
				keys = []string{msg.SrcAddrObj().String()}
			} else if definition.RelevantAddress == "destination" {
				keys = []string{msg.DstAddrObj().String()}
			} else if definition.RelevantAddress == "both" {
				keys = []string{msg.SrcAddrObj().String(), msg.DstAddrObj().String()}
			}
			for _, key := range keys {
				record := definition.Database.GetRecord(key)
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
