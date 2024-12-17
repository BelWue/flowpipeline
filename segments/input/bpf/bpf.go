//go:build linux
// +build linux

package bpf

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/bwNetFlow/flowpipeline/segments"
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
		log.Printf("[error] Bpf: setting the config parameter 'device' is required.")
		return nil
	}

	newsegment.BufferSize = 65536
	if config["buffersize"] != "" {
		if parsedBufferSize, err := strconv.ParseInt(config["buffersize"], 10, 32); err == nil {
			newsegment.BufferSize = int(parsedBufferSize)
			if newsegment.BufferSize <= 0 {
				log.Println("[error] Bpf: Buffer size needs to be at least 1 and will be rounded up to the nearest multiple of the current page size.")
				return nil
			}
		} else {
			log.Println("[error] Bpf: Could not parse 'buffersize' parameter, using default 65536 (64kB).")
		}
	} else {
		log.Println("[info] Bpf: 'buffersize' set to default 65536 (64kB).")
	}

	// setup bpf dumping
	newsegment.dumper = PacketDumper{BufSize: newsegment.BufferSize}

	err := newsegment.dumper.Setup(newsegment.Device)
	if err != nil {
		log.Printf("[error] Bpf: error setting up BPF dumping: %s", err)
		return nil
	}

	// setup flow export
	_, err = time.ParseDuration(config["activetimeout"])
	if err != nil {
		if config["activetimeout"] == "" {
			log.Println("[info] Bpf: 'activetimeout' set to default '30m'.")
		} else {
			log.Println("[warning] Bpf: 'activetimeout' was invalid, fallback to default '30m'.")
		}
		newsegment.ActiveTimeout = "30m"
	} else {
		newsegment.ActiveTimeout = config["activetimeout"]
		log.Printf("[info] Bpf: 'activetimeout' set to '%s'.", config["activetimeout"])
	}

	_, err = time.ParseDuration(config["inactivetimeout"])
	if err != nil {
		if config["inactivetimeout"] == "" {
			log.Println("[info] Bpf: 'inactivetimeout' set to default '15s'.")
		} else {
			log.Println("[warning] Bpf: 'inactivetimeout' was invalid, fallback to default '15s'.")
		}
		newsegment.InactiveTimeout = "15s"
	} else {
		newsegment.ActiveTimeout = config["inactivetimeout"]
		log.Printf("[info] Bpf: 'inactivetimeout' set to '%s'.", config["inactivetimeout"])
	}

	newsegment.exporter, err = NewFlowExporter(newsegment.ActiveTimeout, newsegment.InactiveTimeout)
	if err != nil {
		log.Printf("[error] Bpf: error setting up exporter: %s", err)
		return nil
	}
	return newsegment
}

func (segment *Bpf) Run(wg *sync.WaitGroup) {
	err := segment.dumper.Start()
	if err != nil {
		log.Printf("[error] Bpf: error starting up BPF dumping: %s", err)
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

	log.Printf("[info] Bpf: Startup finished, exporting flows from '%s'", segment.Device)
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
