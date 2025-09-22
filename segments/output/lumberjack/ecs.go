package lumberjack

import (
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/utils"

	//"golang.org/x/net/publicsuffix"
	"net/netip"
	"time"
)

const (
	ElasticCommonSchemaVersion = "8.17"
)

type ECSECS struct {
	Version string `json:"version,omitempty"`
}

type ElasticCommonSchema struct {
	// Base Fields (https://www.elastic.co/guide/en/ecs/current/ecs-principles-implementation.html#_base_fields)
	Timestamp uint64            `json:"@timestamp,omitempty"`
	ECS       *ECSECS           `json:"ecs,omitempty"`
	Message   string            `json:"message,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`

	// ECS Categorization Fields (https://www.elastic.co/guide/en/ecs/current/ecs-category-field-values-reference.html)
	Event ECSEvent `json:"event"`

	// Source (https://www.elastic.co/guide/en/ecs/current/ecs-source.html) and
	// Destination (https://www.elastic.co/guide/en/ecs/current/ecs-destination.html)
	Source      *ECSSourceOrDest `json:"source,omitempty"`
	Destination *ECSSourceOrDest `json:"destination,omitempty"`

	// Network (https://www.elastic.co/docs/reference/ecs/ecs-network)
	Network *ECSNetwork `json:"network,omitempty"`

	// needed for community ID enrichment in Elastic
	ICMP *ECSRelatedICMP `json:"icmp,omitempty"`

	// Related Fields (https://www.elastic.co/guide/en/ecs/current/ecs-related.html)
	Related *ECSRelated `json:"related,omitempty"`
}

type ECSNetwork struct {
	IanaNumber uint32 `json:"iana_number,omitempty"`
	Transport  string `json:"transport,omitempty"`
	Bytes      uint64 `json:"bytes"`
	Packets    uint64 `json:"packets"`
	Type       string `json:"type,omitempty"`
}

// needed for https://www.elastic.co/docs/reference/enrich-processor/community-id-processor
type ECSRelatedICMP struct {
	Type uint32 `json:"type,omitempty"`
	Code uint32 `json:"code,omitempty"`
}

func ECSFromEnrichedFlow(enrichedFlow *pb.EnrichedFlow) *ElasticCommonSchema {
	var (
		ok                          bool
		SourceAS                    *AutonomousSystem = nil
		DestinationAS               *AutonomousSystem = nil
		srcIP                       netip.Addr
		SourceIPString              string
		SourceAddress               = ""
		SourceMAC                   = ""
		SourceDomain                = ""
		SourceRegisteredDomain      = ""
		SourceTopLevelDomain        = ""
		dstIP                       netip.Addr
		DestinationIPString         string
		DestinationAddress          = ""
		DestinationMAC              = ""
		DestinationDomain           = ""
		DestinationRegisteredDomain = ""
		DestinationTopLevelDomain   = ""
		RelatedHosts                = make([]string, 0, 2)
	)
	// source ip & address
	if enrichedFlow.SourceIP != "" { // use IP string from addrstring segment if available
		SourceIPString = enrichedFlow.SourceIP
		SourceAddress = enrichedFlow.SourceIP
		// TODO: set SourceIP
	} else {
		// parse source address
		srcIP, ok = netip.AddrFromSlice(enrichedFlow.SrcAddr)
		if ok {
			SourceIPString = srcIP.String()
			SourceAddress = SourceIPString
		}
	}
	// use hostname from reversedns segment if available
	/*	if enrichedFlow.SrcHostName != "" {
		SourceDomain = enrichedFlow.SrcHostName
		SourceRegisteredDomain, err = publicsuffix.EffectiveTLDPlusOne(enrichedFlow.SrcHostName[:len(enrichedFlow.SrcHostName)-2])
		if err != nil {
			SourceRegisteredDomain = ""
		}
		SourceTopLevelDomain, _ = publicsuffix.PublicSuffix(enrichedFlow.SrcHostName)
		RelatedHosts = append(RelatedHosts, enrichedFlow.SrcHostName)
	}*/
	// add source MAC if set
	if enrichedFlow.SrcMac != 0 {
		if enrichedFlow.SourceMAC != "" {
			SourceMAC = enrichedFlow.SourceMAC
		} else {
			SourceMAC = enrichedFlow.SrcMacString(pb.MACSeparatorDash)
		}
	}

	// destination ip & address
	if enrichedFlow.DestinationIP != "" { // use IP string from addrstring segment if available
		DestinationIPString = enrichedFlow.DestinationIP
		DestinationAddress = enrichedFlow.DestinationIP
		// TODO: set DestinationIP
	} else {
		dstIP, ok = netip.AddrFromSlice(enrichedFlow.DstAddr)
		if ok {
			DestinationIPString = dstIP.String()
			DestinationAddress = DestinationIPString
		}
	}
	// use hostname from reversedns segment if available
	/*	if enrichedFlow.DstHostName != "" {
		DestinationDomain = enrichedFlow.DstHostName
		DestinationRegisteredDomain, err = publicsuffix.EffectiveTLDPlusOne(enrichedFlow.DstHostName[:len(enrichedFlow.DstHostName)-2])
		if err != nil {
			DestinationRegisteredDomain = ""
		}
		DestinationTopLevelDomain, _ = publicsuffix.PublicSuffix(enrichedFlow.DstHostName)
		RelatedHosts = append(RelatedHosts, enrichedFlow.DstHostName)
	}*/
	// add source MAC if set
	if enrichedFlow.DstMac != 0 {
		if enrichedFlow.DestinationMAC != "" {
			DestinationMAC = enrichedFlow.DestinationMAC
		} else {
			DestinationMAC = enrichedFlow.DstMacString(pb.MACSeparatorDash)
		}
	}

	// AS
	srcAS := enrichedFlow.GetSrcAs()
	if srcAS != 0 {
		SourceAS = &AutonomousSystem{
			Number: srcAS,
		}
	}
	dstAS := enrichedFlow.GetDstAs()
	if dstAS != 0 {
		DestinationAS = &AutonomousSystem{
			Number: dstAS,
		}
	}

	var icmp *ECSRelatedICMP = nil
	// only set if proto is ICMPv4 or ICMPv6
	if enrichedFlow.Proto == 1 || enrichedFlow.Proto == 58 {
		icmp = &ECSRelatedICMP{
			Type: enrichedFlow.IcmpType,
			Code: enrichedFlow.IcmpCode,
		}
	}

	var networkType string
	switch enrichedFlow.Etype {
	case 2048:
		networkType = "ipv4"
	case 34525:
		networkType = "ipv6"
	}

	result := &ElasticCommonSchema{
		Timestamp: enrichedFlow.GetTimeFlowStartMs(),
		ECS:       &ECSECS{Version: ElasticCommonSchemaVersion},
		Event: ECSEvent{
			Kind:     ECSEventKindEvent,
			Category: []ECSEventCategory{ECSEventCategoryNetwork},
			Type:     []ECSEventType{ECSEventTypeConnection},
			Outcome:  ECSEventOutcomeSuccess,
			Created:  time.Now().UnixMilli(),
			Start:    enrichedFlow.GetTimeFlowStartMs(),
			End:      enrichedFlow.GetTimeFlowEndMs(),
			Duration: int64(enrichedFlow.GetTimeFlowEndNs() - enrichedFlow.GetTimeFlowStartNs()),
			Provider: ECSEventDefaultProvider,
			Module:   ECSEventDefaultModule,
		},
		Source: &ECSSourceOrDest{
			Address:          SourceAddress,
			Bytes:            enrichedFlow.GetBytes(),
			IP:               SourceIPString,
			MAC:              SourceMAC,
			Packets:          enrichedFlow.GetPackets(),
			Port:             enrichedFlow.GetSrcPort(),
			Domain:           SourceDomain,
			RegisteredDomain: SourceRegisteredDomain,
			//Subdomain:        SourceSubdomain,
			TopLevelDomain:   SourceTopLevelDomain,
			AutonomousSystem: SourceAS,
		},
		Destination: &ECSSourceOrDest{
			Address:          DestinationAddress,
			Bytes:            0, // flows are uni-directional
			IP:               DestinationIPString,
			MAC:              DestinationMAC,
			Packets:          0, // flows are uni-directional
			Port:             enrichedFlow.GetDstPort(),
			Domain:           DestinationDomain,
			RegisteredDomain: DestinationRegisteredDomain,
			//Subdomain:        DestinationSubdomain,
			TopLevelDomain:   DestinationTopLevelDomain,
			AutonomousSystem: DestinationAS,
		},
		ICMP: icmp,
		Network: &ECSNetwork{
			IanaNumber: enrichedFlow.Proto,
			Transport:  utils.IanaProtocolNumberToLowercaseName(enrichedFlow.Proto),
			Bytes:      enrichedFlow.GetBytes(),
			Packets:    enrichedFlow.GetPackets(),
			Type:       networkType,
		},
		Related: &ECSRelated{
			Hash:  nil,
			Hosts: RelatedHosts,
			IP:    []string{SourceIPString, DestinationIPString},
			User:  nil,
		},
	}
	return result
}
