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
// This segment has one configuration option `macseparator`. A value of `dash` will use dashes as separator, otherwise
// colons are used (default).
//
// This segment is intended to be used in conjunction with the `dropfields` segment to remove the original fields.
package addrstrings

import (
	"strings"
	"sync"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

type AddrStrings struct {
	segments.BaseSegment
	Separator pb.MacSeparator
}

func (segment AddrStrings) New(config map[string]string) segments.Segment {
	// default separator is dash
	var sep = pb.MACSeparatorColon
	if strings.Contains(config["macseparator"], "dash") {
		sep = pb.MACSeparatorDash
	}
	return &AddrStrings{
		Separator: sep,
	}
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
			original.SourceMAC = original.SrcMacString(segment.Separator)
		}
		// DestinationMAC
		if original.DstMac != 0x0 {
			original.DestinationMAC = original.DstMacString(segment.Separator)
		}
		segment.Out <- original
	}
}

// register segment
func init() {
	segment := &AddrStrings{}
	segments.RegisterSegment("addrstrings", segment)
}
