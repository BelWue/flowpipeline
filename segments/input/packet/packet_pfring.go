//go:build linux && cgo && pfring
// +build linux,cgo,pfring

package packet

import (
	"github.com/google/gopacket/pfring"
	"github.com/rs/zerolog/log"
)

const cgoEnabled = true
const pfringEnabled = true

func getPfringHandle(source string, filter string) *pfring.Ring {
	var ring *pfring.Ring
	var err error
	if ring, err = pfring.NewRing(source, 65536, pfring.FlagPromisc); err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not setup pfring capture: ")
	} else if err := ring.SetBPFFilter(filter); err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not set BPF filter: ")
	} else if err := ring.Enable(); err != nil {
		log.Fatal().Err(err).Msg("Packet: Could not initiate capture: ")
	}
	return ring
}
