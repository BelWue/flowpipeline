//go:build linux
// +build linux

package bpf

import (
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
)

type Bpf struct {
	segments.BaseSegment

	dumper   PacketDumper
	exporter *FlowExporter

	Device          string // required, the name of the device to capture, e.g. "eth0"
	ActiveTimeout   string // optional, default is 30m
	InactiveTimeout string // optional, default is 15s
	BufferSize      int    // optional, default is 65536 (64kB)
}

func (segment Bpf) New(config map[string]string) segments.Segment {
	newsegment := &Bpf{}

	var ok bool
	newsegment.Device, ok = config["device"]
	if !ok {
		log.Error().Msgf("Bpf: setting the config parameter 'device' is required.")
		return nil
	}

	newsegment.BufferSize = 65536
	if config["buffersize"] != "" {
		if parsedBufferSize, err := strconv.ParseInt(config["buffersize"], 10, 32); err == nil {
			newsegment.BufferSize = int(parsedBufferSize)
			if newsegment.BufferSize <= 0 {
				log.Error().Msg("Bpf: Buffer size needs to be at least 1 and will be rounded up to the nearest multiple of the current page size.")
				return nil
			}
		} else {
			log.Error().Msg("Bpf: Could not parse 'buffersize' parameter, using default 65536 (64kB).")
		}
	} else {
		log.Info().Msg("Bpf: 'buffersize' set to default 65536 (64kB).")
	}

	// setup bpf dumping
	newsegment.dumper = PacketDumper{BufSize: newsegment.BufferSize}

	err := newsegment.dumper.Setup(newsegment.Device)
	if err != nil {
		log.Error().Err(err).Msg("Bpf: error setting up BPF dumping: ")
		return nil
	}

	// setup flow export
	_, err = time.ParseDuration(config["activetimeout"])
	if err != nil {
		if config["activetimeout"] == "" {
			log.Info().Msg("Bpf: 'activetimeout' set to default '30m'.")
		} else {
			log.Warn().Msg("Bpf: 'activetimeout' was invalid, fallback to default '30m'.")
		}
		newsegment.ActiveTimeout = "30m"
	} else {
		newsegment.ActiveTimeout = config["activetimeout"]
		log.Info().Msgf("Bpf: 'activetimeout' set to '%s'.", config["activetimeout"])
	}

	_, err = time.ParseDuration(config["inactivetimeout"])
	if err != nil {
		if config["inactivetimeout"] == "" {
			log.Info().Msg("Bpf: 'inactivetimeout' set to default '15s'.")
		} else {
			log.Warn().Msg("Bpf: 'inactivetimeout' was invalid, fallback to default '15s'.")
		}
		newsegment.InactiveTimeout = "15s"
	} else {
		newsegment.ActiveTimeout = config["inactivetimeout"]
		log.Info().Msgf("Bpf: 'inactivetimeout' set to '%s'.", config["inactivetimeout"])
	}

	newsegment.exporter, err = NewFlowExporter(newsegment.ActiveTimeout, newsegment.InactiveTimeout)
	if err != nil {
		log.Error().Err(err).Msg("Bpf: error setting up exporter: ")
		return nil
	}
	return newsegment
}

func (segment *Bpf) Run(wg *sync.WaitGroup) {
	err := segment.dumper.Start()
	if err != nil {
		log.Error().Err(err).Msg("Bpf: error starting up BPF dumping: ")
		segment.ShutdownParentPipeline()
		return
	}
	segment.exporter.Start(segment.dumper.SamplerAddress)
	go segment.exporter.ConsumeFrom(segment.dumper.Packets())
	defer func() {
		close(segment.Out)
		segment.dumper.Stop()
		wg.Done()
	}()

	log.Info().Msgf("Bpf: Startup finished, exporting flows from '%s'", segment.Device)
	for {
		select {
		case msg, ok := <-segment.exporter.Flows:
			if !ok {
				return
			}
			segment.Out <- msg
		case msg, ok := <-segment.In:
			if !ok {
				segment.exporter.Stop()
				return
			}
			segment.Out <- msg
		}
	}
}

func init() {
	segment := &Bpf{}
	segments.RegisterSegment("bpf", segment)
}
