// The `addrstrings` segment adds string representations of IP and MAC addresses which
// are set. The new fields are
//
// * `SourceIP` (from `SrcAddr`)
// * `DestinationIP` (from `DstAddr`)
// * `NextHopIP` (from `NextHop`)
// * `SamplerIP` (from `SamplerAddress`)
//
// * `SourceMAC` (from `SrcMac`)
// * `DestinationMAC` (from `DstMac`)
//
// This segment has no configuration options. It is intended to be used in conjunction
// with the `dropfields` segment to remove the original fields.
package addrstrings

import (
	"github.com/BelWue/flowpipeline/segments"
	"sync"
)

type AddrStrings struct {
	segments.BaseSegment
}

func (segment AddrStrings) New(config map[string]string) segments.Segment {
	return &AddrStrings{}
}

func (segment *AddrStrings) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	for original := range segment.In {
		// SourceIP
		if original.SrcAddr != nil {
			original.SourceIP = original.SrcAddrObj().String()
		}
		// DestinationIP
		if original.DstAddr != nil {
			original.DestinationIP = original.DstAddrObj().String()
		}
		// NextHopIP
		if original.NextHop != nil {
			original.NextHopIP = original.NextHopObj().String()
		}
		// SamplerIP
		if original.SamplerAddress != nil {
			original.SamplerIP = original.SamplerAddressObj().String()
		}
		// SourceMAC
		if original.SrcMac != 0x0 {
			original.SourceMAC = original.SrcMacString()
		}
		// DestinationMAC
		if original.DstMac != 0x0 {
			original.DestinationMAC = original.DstMacString()
		}
		segment.Out <- original
	}
}

// register segment
func init() {
	segment := &AddrStrings{}
	segments.RegisterSegment("addrstrings", segment)
}
