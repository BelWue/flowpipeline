package pb

import (
	"fmt"
	"net"
)

type MacSeparator int

const (
	MACSeparatorColon MacSeparator = iota
	MACSeparatorDash  MacSeparator = iota
)

const (
	macSeparatorFormatColon = "%02x:%02x:%02x:%02x:%02x:%02x"
	macSeparatorFormatDash  = "%02x-%02x-%02x-%02x-%02x-%02x"
)

var (
	FlowDirectionMap = map[uint32]string{
		0: "Incoming",
		1: "Outgoing"}
	EtypeMap = map[uint32]string{
		0x0800: "IPv4",
		0x0806: "ARP",
		0x0842: "Wake-on-LAN",
		0x86DD: "IPv6",
		0x8809: "Ethernet Slow Protocols (LACP)",
		0x8847: "MPLS unicast",
		0x8848: "MPLS multicast",
		0x8863: "PPPoE Discovery Stage",
		0x8864: "PPPoE Session Stage",
		0x889A: "HyperSCSI (SCSI over Ethernet)",
		0x88A2: "ATA over Ethernet",
		0x88A4: "EtherCAT Protocol",
		0x88CC: "LLDP",
		0x88E5: "MAC Security",
		0x8906: "Fibre Channel over Ethernet (FCoE)",
		0x8914: "FCoE Initialization Protocol",
		0x9000: "Ethernet Configuration Testing Protocol"}
	ForwardingStatusMap = map[uint32]string{
		0:   "Unknown",
		64:  "Forwarded (Unknown)",
		65:  "Forwarded (Fragmented)",
		66:  "Forwarded (Not Fragmented)",
		128: "Dropped (Unknown)",
		129: "Dropped (ACL Deny)",
		130: "Dropped (ACL Drop)",
		131: "Dropped (Unroutable)",
		132: "Dropped (Adjacency)",
		133: "Dropped (Fragmented and DF set)",
		134: "Dropped (Bad Header Checksum)",
		135: "Dropped (Bad Total Length)",
		136: "Dropped (Bad Header Length)",
		137: "Dropped (Bad TTL)",
		138: "Dropped (Policer)",
		139: "Dropped (WRED)",
		140: "Dropped (RPF)",
		141: "Dropped (For Us)",
		142: "Dropped (Bad Output Interface)",
		143: "Dropped (Hardware)",
		192: "Consumed (Unknown)",
		193: "Consumed (Terminate Punt Adjacency)",
		194: "Consumed (Terminate Incomplete Adjacency)",
		195: "Consumed (Terminate For Us)"}
)

func (flow *EnrichedFlow) FlowDirectionString() string {
	return FlowDirectionMap[flow.GetFlowDirection()]
}

func (flow *EnrichedFlow) IsIncoming() bool {
	return flow.GetFlowDirection() == 0
}

func (flow *EnrichedFlow) IsOutgoing() bool {
	return flow.GetFlowDirection() == 1
}

func (flow *EnrichedFlow) Peer() string {
	switch flow.GetFlowDirection() {
	case 0:
		return flow.GetSrcIfDesc()
	case 1:
		return flow.GetDstIfDesc()
	default:
		return ""
	}
}

func (flow *EnrichedFlow) EtypeString() string {
	return EtypeMap[flow.GetEtype()]
}

func (flow *EnrichedFlow) IPVersion() uint8 {
	switch flow.GetEtype() {
	case 0x0800:
		return 4
	case 0x86dd:
		return 6
	default:
		return 0
	}
}

func (flow *EnrichedFlow) IPVersionString() string {
	if flow.GetEtype() == 0x800 || flow.GetEtype() == 0x86dd {
		return EtypeMap[flow.GetEtype()]
	} else {
		return ""
	}
}

func (flow *EnrichedFlow) IsIPv4() bool {
	return flow.GetEtype() == 0x0800
}

func (flow *EnrichedFlow) IsIPv6() bool {
	return flow.GetEtype() == 0x86dd
}

func (flow *EnrichedFlow) ForwardingStatusString() string {
	return ForwardingStatusMap[flow.GetForwardingStatus()]
}

func (flow *EnrichedFlow) IsConsumed() bool {
	return 192 <= flow.GetForwardingStatus() // && < 256
}

func (flow *EnrichedFlow) IsDropped() bool {
	return 128 <= flow.GetForwardingStatus() && flow.GetForwardingStatus() < 192
}

func (flow *EnrichedFlow) IsForwarded() bool {
	return 64 <= flow.GetForwardingStatus() && flow.GetForwardingStatus() < 128
}

func (flow *EnrichedFlow) IsUnknownForwardingStatus() bool {
	return flow.GetForwardingStatus() < 64
}

func (flow *EnrichedFlow) SrcAddrObj() net.IP {
	return net.IP(flow.SrcAddr)
}

func (flow *EnrichedFlow) DstAddrObj() net.IP {
	return net.IP(flow.DstAddr)
}

func (flow *EnrichedFlow) NextHopObj() net.IP {
	return net.IP(flow.NextHop)
}

func (flow *EnrichedFlow) SamplerAddressObj() net.IP {
	return net.IP(flow.SamplerAddress)
}

func (flow *EnrichedFlow) SrcMacString(sep MacSeparator) string {
	return MacToString(flow.SrcMac, sep)
}

func (flow *EnrichedFlow) DstMacString(sep MacSeparator) string {
	return MacToString(flow.DstMac, sep)
}

func MacToString(mac uint64, sep MacSeparator) string {
	// this code duplication is needed to get good performance by not requiring any allocations
	// or dynamic string building
	switch sep {
	case MACSeparatorDash:
		return fmt.Sprintf(macSeparatorFormatDash,
			(mac & 0x0000000000FF),
			(mac&0x00000000FF00)>>8,
			(mac&0x000000FF0000)>>16,
			(mac&0x0000FF000000)>>24,
			(mac&0x00FF00000000)>>32,
			(mac&0xFF0000000000)>>40,
		)
	// colon is default
	default:
		return fmt.Sprintf(macSeparatorFormatColon,
			(mac & 0x0000000000FF),
			(mac&0x00000000FF00)>>8,
			(mac&0x000000FF0000)>>16,
			(mac&0x0000FF000000)>>24,
			(mac&0x00FF00000000)>>32,
			(mac&0xFF0000000000)>>40,
		)
	}
}

func (flow *EnrichedFlow) GetBps() uint64 {
	duration := flow.TimeFlowEnd - flow.TimeFlowStart
	if duration == 0 {
		duration = 1
	}
	bpns := flow.Bytes / duration
	return bpns
}

func (flow *EnrichedFlow) GetPps() uint64 {
	duration := flow.TimeFlowEnd - flow.TimeFlowStart
	if duration == 0 {
		duration = 1
	}
	ppns := flow.Packets / duration
	return ppns
}
