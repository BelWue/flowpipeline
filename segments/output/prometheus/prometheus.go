// Collects and serves statistics about flows.
// Configuration options for this segments are:
// The endoint field can be configured:
// <host>:8080/flowdata
// <host>:8080/metrics
// The labels to be exported can be set in the configuration.
// Default labels are:
//
//	Etype,Proto
//
// Additional Labels can be defined as long as they match a field in flowpipeline/pb/enrichedflow.pb.go:EnrichedFlow
// These include :
//
// # SrcAs,DstAs,Proto,DstPort,Bytes,Packets,SamplerAddress,ForwardingStatus,TimeReceived,TimeReceivedNs,TimeFlowStart,TimeFlowStartNs
//
// The additional label should be used with care, because of infite quantity.
package prometheus

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/go-co-op/gocron/v2"
)

type Prometheus struct {
	segments.BaseSegment
	Endpoint          string         // optional, default value is ":8080"
	MetricsPath       string         // optional, default is "/metrics"
	FlowdataPath      string         // optional, default is "/flowdata"
	Labels            []string       // optional, list of labels to be exported
	VacuumInterval    *time.Duration // optional, intervall in which counters should be reset (can lead to dataloss)
	ExportASPathPairs bool           // optional, if true, as path pairs will be exported
	ExportASPaths     bool           // optional, if true, as paths will be exported

	PromExporter *Exporter
}

func (segment Prometheus) New(config map[string]string) segments.Segment {
	var endpoint string = ":8080"
	if config["endpoint"] == "" {
		log.Info().Msg("Prometheus: Missing configuration parameter 'endpoint'. Using default port ':8080'")
	} else {
		endpoint = config["endpoint"]
	}
	var metricsPath string = "/metrics"
	if config["metricspath"] == "" {
		log.Info().Msg("Prometheus: Missing configuration parameter 'metricspath'. Using default path '/metrics'")
	} else {
		metricsPath = config["metricspath"]
	}
	var flowdataPath string = "/flowdata"
	if config["flowdatapath"] == "" {
		log.Info().Msg("Prometheus: Missing configuration parameter 'flowdatapath'. Using default path '/flowdata'")
	} else {
		flowdataPath = config["flowdatapath"]
	}
	var vacuumInterval *time.Duration
	if config["vacuum_interval"] != "" {
		vacuumIntervalDuration, err := time.ParseDuration(config["vacuum_interval"])
		if err != nil {
			log.Warn().Msg("Prometheus: Failed to parse vacuum intervall '" + config["vacuum_interval"] + "' - continuing without vacuum interval")
		} else {
			log.Info().Msg("Prometheus: Setting prometheus vacuum interval to " + config["vacuum_interval"] + " this will lead to data loss of up to one scraping intervall!")
			vacuumInterval = &vacuumIntervalDuration
		}
	}
	var exportASPathPairs bool = false
	if config["export_as_pairs"] == "" {
		log.Info().Msg("Prometheus: Missing configuration parameter 'export_as_pairs'. Using default value 'false'")
	} else if strings.ToLower(config["export_as_pairs"]) == "true" {
		log.Info().Msg("Prometheus: Export of AS path pairs is enabled")
		exportASPathPairs = true
	}

	var exportASPaths bool = false
	if config["export_as_paths"] == "" {
		log.Info().Msg("Prometheus: Missing configuration parameter 'export_as_paths'. Using default value 'false'")
	} else if strings.ToLower(config["export_as_paths"]) == "true" {
		log.Info().Msg("Prometheus: Export of AS paths is enabled")
		exportASPaths = true
	}

	newsegment := &Prometheus{
		Endpoint:          endpoint,
		MetricsPath:       metricsPath,
		FlowdataPath:      flowdataPath,
		VacuumInterval:    vacuumInterval,
		ExportASPathPairs: exportASPathPairs,
		ExportASPaths:     exportASPaths,
	}

	// set default labels if not configured
	var labels []string
	if config["labels"] == "" {
		log.Info().Msg("Prometheus: Configuration parameter 'labels' not set. Using default labels 'Etype,Proto' to export")
		labels = strings.Split("Etype,Proto", ",")
	} else {
		labels = strings.Split(config["labels"], ",")
	}
	protofields := reflect.TypeOf(pb.EnrichedFlow{})
	for _, field := range labels {
		field = strings.TrimSpace(field)
		_, found := protofields.FieldByName(field)
		if !found {
			log.Error().Msgf("Prometheus: Field '%s' specified in 'labels' does not exist.", field)
			return nil
		}
		newsegment.Labels = append(newsegment.Labels, field)
	}
	return newsegment
}

func (segment *Prometheus) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	segment.PromExporter = &Exporter{}
	segment.initializeExporter(segment.PromExporter)
	if segment.VacuumInterval != nil {
		segment.AddVacuumCronJob(segment.PromExporter)
	}

	for msg := range segment.In {
		labelset := make(map[string]string)
		values := reflect.ValueOf(msg).Elem()
		for _, fieldname := range segment.Labels {
			value := values.FieldByName(fieldname).Interface()
			switch value.(type) {
			case []uint8: // this is necessary for proper formatting
				ipstring := net.IP(value.([]uint8)).String()
				if ipstring == "<nil>" {
					ipstring = ""
				}
				labelset[fieldname] = ipstring
			case uint32: // this is because FormatUint is much faster than Sprint
				labelset[fieldname] = strconv.FormatUint(uint64(value.(uint32)), 10)
			case uint64: // this is because FormatUint is much faster than Sprint
				labelset[fieldname] = strconv.FormatUint(uint64(value.(uint64)), 10)
			case string: // this is because doing nothing is also much faster than Sprint
				labelset[fieldname] = value.(string)
			default:
				labelset[fieldname] = fmt.Sprint(value)
			}
		}
		segment.PromExporter.Increment(msg.Bytes, msg.Packets, labelset)

		if segment.ExportASPathPairs && segment.PromExporter != nil {
			segment.PromExporter.ExportASPathPairsWithDirection(msg.SrcAsPath, "Source", msg)
			segment.PromExporter.ExportASPathPairsWithDirection(msg.DstAsPath, "Destination", msg)
		}
		if segment.ExportASPaths {
			segment.PromExporter.ExportASPaths(msg)
		}
		segment.Out <- msg
	}
}

func (segment *Prometheus) initializeExporter(exporter *Exporter) {
	exporter.Initialize(segment.Labels)
	exporter.ServeEndpoints(segment)
}

func (segment *Prometheus) AddVacuumCronJob(promExporter *Exporter) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		log.Warn().Err(err).Msg("Prometheus: Failed initializing prometheus exporter vacuum job")
	}
	_, err = scheduler.NewJob(
		gocron.DurationJob(*segment.VacuumInterval),
		gocron.NewTask(promExporter.ResetCounter),
	)
	if err != nil {
		log.Warn().Err(err).Msg("Prometheus: Failed initializing prometheus exporter vacuum job")
	}
	// start the scheduler
	scheduler.Start()
}

func (e *Exporter) ExportASPaths(flow *pb.EnrichedFlow) {
	asPath := DedupConsecutiveASNs(flow.AsPath)
	if len(asPath) < 2 {
		return
	}
	e.flowAsPathBytes.WithLabelValues(fmt.Sprint(asPath)).Add(float64(flow.Bytes))
}

func (e *Exporter) ExportASPathPairsWithDirection(asPath []uint32, direction string, flow *pb.EnrichedFlow) {
	asPath = DedupConsecutiveASNs(asPath)
	if len(asPath) < 2 {
		return
	}

	if direction == "Source" {
		start := fmt.Sprint(asPath[0])
		e.flowAsPairsBytes.WithLabelValues(start, start, direction).Add(float64(flow.Bytes))

		for i := 0; i < len(asPath)-1; i++ {
			from := fmt.Sprint(asPath[i])
			to := fmt.Sprint(asPath[i+1])
			e.flowAsPairsBytes.WithLabelValues(from, to, direction).Add(float64(flow.Bytes))
		}
	}

	if direction == "Destination" {
		for i := 0; i < len(asPath)-1; i++ {
			from := fmt.Sprint(asPath[i])
			to := fmt.Sprint(asPath[i+1])
			e.flowAsPairsBytes.WithLabelValues(from, to, direction).Add(float64(flow.Bytes))
		}

		end := fmt.Sprint(asPath[len(asPath)-1])
		e.flowAsPairsBytes.WithLabelValues(end, end, direction).Add(float64(flow.Bytes))
	}
}

func DedupConsecutiveASNs(asPath []uint32) []uint32 {
	if len(asPath) == 0 {
		return nil
	}
	deduped := []uint32{asPath[0]}
	for _, asn := range asPath[1:] {
		if asn != deduped[len(deduped)-1] {
			deduped = append(deduped, asn)
		}
	}
	return deduped
}

func init() {
	segment := &Prometheus{}
	segments.RegisterSegment("prometheus", segment)
}
