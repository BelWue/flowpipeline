// The `protomap` segment sets the ProtoName string field according to the Proto
// integer field. Note that this should only be done before final usage, as
// lugging additional string content around can be costly regarding performance
// and storage size.
package protomap

import (
	"sync"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/utils"
)

type Protomap struct {
	segments.BaseSegment
}

// TODO make configurable to only add specific protocol names instead of all
func (segment Protomap) New(config map[string]string) segments.Segment {
	return &Protomap{}
}

func (segment *Protomap) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		proto := utils.IanaProtocolNumberToName(msg.Proto)
		if proto == "" {
			proto = "UNKNOWN"
		}
		msg.ProtoName = proto

		segment.Out <- msg
	}
}

func init() {
	segment := &Protomap{}
	segments.RegisterSegment("protomap", segment)
}
