//go:build linux && cgo
// +build linux,cgo

package packet

import (
	"github.com/rs/zerolog/log"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const cgoEnabled = true
const pfringEnabled = false

func getPcapHandle(source string, filter string) *pcap.Handle {
	inactive, err := pcap.NewInactiveHandle(source)
	if err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not setup libpcap capture: ")
	}
	defer inactive.CleanUp()

	handle, err := inactive.Activate()
	if err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not initiate capture: ")
	}

	err = handle.SetBPFFilter(filter)
	if err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not set BPF filter: ")
	}
	return handle
}

func getPcapFile(source string, filter string) *pcap.Handle {
	var handle *pcap.Handle
	var err error
	if handle, err = pcap.OpenOffline(source); err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not setup pcap reader")
	} else if err := handle.SetBPFFilter(filter); err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not set BPF filter")
	}
	if err != nil {
		log.Fatal().Err(err).Msg("Packet: Failed opening pcap file")
	}
	return handle
}

type dummyHandle interface {
	LinkType() layers.LinkType
	Close()
	ReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error)
}

var getPfringHandle func(source string, filter string) dummyHandle
