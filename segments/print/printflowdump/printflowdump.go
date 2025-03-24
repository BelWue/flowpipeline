// Prints all incoming flows in a specific flowdump format.
// Add the protomap segment before this segment to enrich any flow message
// with human readable protocol names instead of the protocol numbers.
// This segment currently has no way to configure the output format (TODO).
package printflowdump

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/modify/protomap"
	"github.com/dustin/go-humanize"
)

type PrintFlowdump struct {
	segments.BaseSegment
	UseProtoname bool // optional, default is true
	Verbose      bool // optional, default is false
	Highlight    bool // optional, default is false
}

func (segment *PrintFlowdump) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		fmt.Println("\033[0m") // reset color in case we're still highlighting
		wg.Done()
	}()
	for msg := range segment.In {
		fmt.Println(segment.format_flow(msg))
		segment.Out <- msg
	}
}

func (segment PrintFlowdump) New(config map[string]string) segments.Segment {
	var useProtoname bool = true
	if config["useprotoname"] != "" {
		if parsedUseProtoname, err := strconv.ParseBool(config["useprotoname"]); err == nil {
			useProtoname = parsedUseProtoname
		} else {
			log.Error().Msg("PrintFlowdump: Could not parse 'useprotoname' parameter, using default true.")
		}
	} else {
		log.Info().Msg("PrintFlowdump: 'useprotoname' set to default true.")
	}

	var verbose bool = false
	if config["verbose"] != "" {
		if parsedVerbose, err := strconv.ParseBool(config["verbose"]); err == nil {
			verbose = parsedVerbose
		} else {
			log.Error().Msg("PrintFlowdump: Could not parse 'verbose' parameter, using default false.")
		}
	} else {
		log.Info().Msg("PrintFlowdump: 'verbose' set to default false.")
	}

	var highlight bool = false
	if config["highlight"] != "" {
		if parsedHighlight, err := strconv.ParseBool(config["highlight"]); err == nil {
			highlight = parsedHighlight
		} else {
			log.Error().Msg("PrintFlowdump: Could not parse 'highlight' parameter, using default false.")
		}
	} else {
		log.Info().Msg("PrintFlowdump: 'highlight' set to default false.")
	}

	return &PrintFlowdump{UseProtoname: useProtoname, Verbose: verbose, Highlight: highlight}

}

func (segment PrintFlowdump) format_flow(flowmsg *pb.EnrichedFlow) string {
	timestamp := time.Unix(int64(flowmsg.TimeFlowEnd), 0).Format("15:04:05")
	src := net.IP(flowmsg.SrcAddr).String()
	dst := net.IP(flowmsg.DstAddr).String()
	if segment.Verbose {
		if flowmsg.SrcHostName != "" {
			src = fmt.Sprintf("%s%s", flowmsg.SrcHostName, src)
		}
		if flowmsg.DstHostName != "" {
			dst = fmt.Sprintf("%s%s", flowmsg.DstHostName, dst)
		}
	}
	router := net.IP(flowmsg.SamplerAddress)

	var srcas, dstas string
	if segment.Verbose {
		if flowmsg.SrcAs != 0 {
			srcas = fmt.Sprintf("AS%d/", flowmsg.SrcAs)
		}
		if flowmsg.DstAs != 0 {
			dstas = fmt.Sprintf("AS%d/", flowmsg.DstAs)
		}
	}
	var proto string
	if segment.UseProtoname {
		if flowmsg.ProtoName != "" {
			proto = flowmsg.ProtoName
		} else {
			// use function from another segment, as it is just a lookup.
			proto = protomap.ProtoNumToString(flowmsg.Proto)
		}
		if proto == "ICMP" && flowmsg.DstPort != 0 {
			proto = fmt.Sprintf("ICMP (type %d, code %d)", flowmsg.DstPort/256, flowmsg.DstPort%256)
		}
	} else {
		proto = fmt.Sprint(flowmsg.Proto)
	}

	duration := flowmsg.TimeFlowEnd - flowmsg.TimeFlowStart
	if duration == 0 {
		duration += 1
	}

	var statusString string
	switch flowmsg.ForwardingStatus & 0b11000000 {
	case 0b00000000:
		statusString = fmt.Sprintf("UNKNOWN/%d", flowmsg.ForwardingStatus)
	case 0b01000000:
		if segment.Verbose {
			switch flowmsg.ForwardingStatus {
			case 64:
				statusString = fmt.Sprintf("FORWARD/%d (regular)", flowmsg.ForwardingStatus)
			case 65:
				statusString = fmt.Sprintf("FORWARD/%d (fragmented)", flowmsg.ForwardingStatus)
			case 66:
				statusString = fmt.Sprintf("FORWARD/%d (not fragmented)", flowmsg.ForwardingStatus)
			}
		} else {
			if flowmsg.ForwardingStatus != 64 {
				statusString = fmt.Sprintf("FORWARD/%d", flowmsg.ForwardingStatus)
			}
		}
	case 0b10000000:
		if segment.Verbose {
			switch flowmsg.ForwardingStatus {
			case 128:
				statusString = fmt.Sprintf("DROP/%d (unknown)", flowmsg.ForwardingStatus)
			case 129:
				statusString = fmt.Sprintf("DROP/%d (ACL deny)", flowmsg.ForwardingStatus)
			case 130:
				statusString = fmt.Sprintf("DROP/%d (ACL drop)", flowmsg.ForwardingStatus)
			case 131:
				statusString = fmt.Sprintf("DROP/%d (unroutable)", flowmsg.ForwardingStatus)
			case 132:
				statusString = fmt.Sprintf("DROP/%d (adjacency)", flowmsg.ForwardingStatus)
			case 133:
				statusString = fmt.Sprintf("DROP/%d (fragmentation but DF set)", flowmsg.ForwardingStatus)
			case 134:
				statusString = fmt.Sprintf("DROP/%d (bad header checksum)", flowmsg.ForwardingStatus)
			case 135:
				statusString = fmt.Sprintf("DROP/%d (bad total length)", flowmsg.ForwardingStatus)
			case 136:
				statusString = fmt.Sprintf("DROP/%d (bad header length)", flowmsg.ForwardingStatus)
			case 137:
				statusString = fmt.Sprintf("DROP/%d (bad TTL)", flowmsg.ForwardingStatus)
			case 138:
				statusString = fmt.Sprintf("DROP/%d (policer)", flowmsg.ForwardingStatus)
			case 139:
				statusString = fmt.Sprintf("DROP/%d (WRED)", flowmsg.ForwardingStatus)
			case 140:
				statusString = fmt.Sprintf("DROP/%d (RPF)", flowmsg.ForwardingStatus)
			case 141:
				statusString = fmt.Sprintf("DROP/%d (for us)", flowmsg.ForwardingStatus)
			case 142:
				statusString = fmt.Sprintf("DROP/%d (bad output interface)", flowmsg.ForwardingStatus)
			case 143:
				statusString = fmt.Sprintf("DROP/%d (hardware)", flowmsg.ForwardingStatus)
			}
		} else {
			statusString = fmt.Sprintf("DROP/%d", flowmsg.ForwardingStatus)
		}
	case 0b11000000:
		if segment.Verbose {
			switch flowmsg.ForwardingStatus {
			case 192:
				statusString = fmt.Sprintf("CONSUMED/%d (unknown)", flowmsg.ForwardingStatus)
			case 193:
				statusString = fmt.Sprintf("CONSUMED/%d (terminate punt adjacency)", flowmsg.ForwardingStatus)
			case 194:
				statusString = fmt.Sprintf("CONSUMED/%d (terminate incomplete adjacency)", flowmsg.ForwardingStatus)
			case 195:
				statusString = fmt.Sprintf("CONSUMED/%d (terminate for us)", flowmsg.ForwardingStatus)
			}
		} else {
			statusString = fmt.Sprintf("CONSUMED/%d", flowmsg.ForwardingStatus)
		}
	}

	var srcIfDesc string
	if flowmsg.SrcIfDesc != "" {
		srcIfDesc = flowmsg.SrcIfDesc
	} else {
		srcIfDesc = fmt.Sprint(flowmsg.InIf)
	}
	var dstIfDesc string
	if flowmsg.DstIfDesc != "" {
		dstIfDesc = flowmsg.DstIfDesc
	} else {
		dstIfDesc = fmt.Sprint(flowmsg.OutIf)
	}

	var color string
	if segment.Highlight {
		color = "\033[31m"
	} else {
		color = "\033[0m"
	}

	var note string
	if flowmsg.Note != "" {
		note = " - " + flowmsg.Note
	}

	return fmt.Sprintf("%s%s: %s%s:%d → %s%s:%d [%s → %s@%s → %s], %s, %ds, %s, %s%s",
		color, timestamp, srcas, src, flowmsg.SrcPort, dstas, dst,
		flowmsg.DstPort, srcIfDesc, statusString, router, dstIfDesc,
		proto, duration,
		humanize.SI(float64(flowmsg.Bytes*8/duration), "bps"),
		humanize.SI(float64(flowmsg.Packets/duration), "pps"),
		note,
	)
}

func init() {
	segment := &PrintFlowdump{}
	segments.RegisterSegment("printflowdump", segment)
}
