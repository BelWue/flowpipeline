// The `goflow` segment provides a convenient interface for
// [goflow2](https://github.com/netsampler/goflow2) right from flowpipeline
// config. It behaves closely like a regular goflow2 instance but provides flows
// directly in our extended format (which has been based on goflow's from the
// beginning of this project).
//
// This flow collector needs to receive input from any IPFIX/Netflow/sFlow
// exporters, for instance your network devices.
package goflow

import (
	"bytes"
	"context"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"google.golang.org/protobuf/encoding/protodelim"

	"github.com/netsampler/goflow2/v2/utils/debug"

	"github.com/netsampler/goflow2/v2/transport"
	_ "github.com/netsampler/goflow2/v2/transport/file"
	_ "github.com/netsampler/goflow2/v2/transport/kafka"

	"github.com/netsampler/goflow2/v2/metrics"
	"github.com/netsampler/goflow2/v2/utils"

	// various formatters
	"github.com/netsampler/goflow2/v2/format"
	_ "github.com/netsampler/goflow2/v2/format/binary"

	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
)

type Goflow struct {
	segments.BaseSegment
	Listen     []url.URL // optional, default config value for this slice is "sflow://:6343,netflow://:2055"
	Workers    uint64    // optional, amount of workers to spawn for each endpoint, default is 1
	Blocking   bool      //optional, default is false
	QueueSize  int       //default is 1000000
	NumSockets int       //default is 1
	goflow_in  chan *pb.EnrichedFlow
}

func (segment Goflow) New(config map[string]string) segments.Segment {

	var listen = "sflow://:6343,netflow://:2055"
	if config["listen"] != "" {
		listen = config["listen"]
	}

	var listenAddressesSlice []url.URL
	for _, listenAddress := range strings.Split(listen, ",") {
		listenAddrUrl, err := url.Parse(listenAddress)
		if err != nil {
			log.Error().Err(err).Msgf("Goflow: error parsing listenAddresses")
			return nil
		}
		// Check if given Port can be parsed to int
		_, err = strconv.ParseUint(listenAddrUrl.Port(), 10, 64)
		if err != nil {
			log.Error().Msgf("Goflow: Port %s could not be converted to integer", listenAddrUrl.Port())
			return nil
		}

		switch listenAddrUrl.Scheme {
		case "netflow", "sflow", "nfl":
			log.Info().Msgf("Goflow: Scheme %s supported.", listenAddrUrl.Scheme)
		default:
			log.Error().Msgf("Goflow: Scheme %s not supported.", listenAddrUrl.Scheme)
			return nil
		}

		listenAddressesSlice = append(listenAddressesSlice, *listenAddrUrl)
	}
	log.Info().Msgf("Goflow: Configured for for %s", listen)

	var workers uint64 = 1
	if config["workers"] != "" {
		if parsedWorkers, err := strconv.ParseUint(config["workers"], 10, 32); err == nil {
			workers = parsedWorkers
			if workers == 0 {
				log.Error().Msg("Goflow: Limiting workers to 0 will not work. Remove this segment or use a higher value >= 1.")
				return nil
			}
		} else {
			log.Error().Msg("Goflow: Could not parse 'workers' parameter, using default 1.")
		}
	} else {
		log.Info().Msg("Goflow: 'workers' set to default '1'.")
	}

	return &Goflow{
		Listen:  listenAddressesSlice,
		Workers: workers,
	}
}

func (segment *Goflow) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	segment.goflow_in = make(chan *pb.EnrichedFlow)
	segment.startGoFlow(&channelDriver{segment.goflow_in})
	for {
		select {
		case msg, ok := <-segment.goflow_in:
			if !ok {
				// do not return here, as this might leave the
				// segment.In channel blocking in our
				// predecessor segment
				segment.goflow_in = nil // make unavailable for select
				// TODO: think about restarting goflow?
			}
			segment.Out <- msg
		case msg, ok := <-segment.In:
			if !ok {
				return
			}
			segment.Out <- msg
		}
	}
}

type channelDriver struct {
	out chan *pb.EnrichedFlow
}

func (d *channelDriver) Send(key, data []byte) error {
	msg := new(pb.ProtoProducerMessage)
	// TODO: can we shave of this Unmarshal here by writing a custom formatter
	if err := protodelim.UnmarshalFrom(bytes.NewReader(data), msg); err != nil {
		log.Error().Msg("Goflow: Conversion error for received flow.")
		return nil
	}
	d.out <- &msg.EnrichedFlow
	return nil
}

func (d *channelDriver) Close(context.Context) error {
	close(d.out)
	return nil
}

func (segment *Goflow) startGoFlow(transport transport.TransportInterface) {
	formatter, err := format.FindFormat("bin")
	if err != nil {
		log.Fatal().Err(err).Msg("Goflow: Failed loading formatter")
		segment.ShutdownParentPipeline()
	}
	var pipes []utils.FlowPipe

	for _, listenAddrUrl := range segment.Listen {
		go func(listenAddrUrl url.URL) {
			var err error

			hostname := listenAddrUrl.Hostname()
			portU64, _ := strconv.ParseUint(listenAddrUrl.Port(), 10, 64)
			if portU64 > math.MaxInt {
				log.Fatal().Err(err).Msg("Goflow: Port out of range")
				segment.ShutdownParentPipeline()
				return
			}
			port := int(portU64)

			if segment.NumSockets == 0 {
				segment.NumSockets = 1
			}

			if segment.QueueSize == 0 {
				segment.QueueSize = 1000000
			}

			cfg := &utils.UDPReceiverConfig{
				Sockets:          segment.NumSockets,
				Workers:          int(segment.Workers),
				QueueSize:        segment.QueueSize,
				Blocking:         segment.Blocking,
				ReceiverCallback: metrics.NewReceiverMetric(),
			}
			recv, err := utils.NewUDPReceiver(cfg)
			if err != nil {
				log.Fatal().Err(err).Msg("Goflow: Failed creating UDP receiver")
				segment.ShutdownParentPipeline()
				return
			}

			var cfgProducer = &protoproducer.ProducerConfig{}
			cfgm, err := cfgProducer.Compile()
			if err != nil {
				log.Fatal().Err(err).Msg("Goflow: Failed compiling ProtoProducerConfig")
			}
			flowProducer, err := protoproducer.CreateProtoProducer(cfgm, protoproducer.CreateSamplingSystem)
			if err != nil {
				log.Fatal().Err(err).Msg("Goflow: Failed creating proto producer")
				segment.ShutdownParentPipeline()
			}

			cfgPipe := &utils.PipeConfig{
				Format:           formatter,
				Transport:        transport,
				Producer:         flowProducer,
				NetFlowTemplater: metrics.NewDefaultPromTemplateSystem, // wrap template system to get Prometheus info
			}

			var pipeline utils.FlowPipe
			switch scheme := listenAddrUrl.Scheme; scheme {
			case "netflow":
				pipeline = utils.NewNetFlowPipe(cfgPipe)
				log.Info().Msgf("Goflow: Listening for Netflow v9 on port %d...", port)
			case "sflow":
				pipeline = utils.NewSFlowPipe(cfgPipe)
				log.Info().Msgf("Goflow: Listening for sflow on port %d...", port)
			case "flow":
				pipeline = utils.NewFlowPipe(cfgPipe)
				log.Info().Msgf("Goflow: Listening for netflow legacy on port %d...", port)
			default:
				log.Fatal().Msgf("Goflow: scheme '%s' does not exist", listenAddrUrl.Scheme)
			}

			decodeFunc := pipeline.DecodeFlow
			// intercept panic and generate error
			decodeFunc = debug.PanicDecoderWrapper(decodeFunc)
			// wrap decoder with Prometheus metrics
			decodeFunc = metrics.PromDecoderWrapper(decodeFunc, listenAddrUrl.Scheme)
			pipes = append(pipes, pipeline)

			err = recv.Start(hostname, port, decodeFunc)

			if err != nil {
				log.Fatal().Err(err).Msg("Goflow: Failed starting goflow receiver")
			}

		}(listenAddrUrl)
	}
}

func init() {
	segment := &Goflow{}
	segments.RegisterSegment("goflow", segment)
}
