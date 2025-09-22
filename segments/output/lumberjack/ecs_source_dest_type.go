package lumberjack

import (
	"encoding/json"
	"fmt"
	"net/netip"
)

type IPAddress netip.Addr

func (ip *IPAddress) MarshalJSON() ([]byte, error) {
	ipString := netip.Addr(*ip).String()
	return json.Marshal(ipString)
}

func (ip *IPAddress) String() string {
	return netip.Addr(*ip).String()
}

type MAC uint64

func (mac *MAC) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%02x-%02x-%02x-%02x-%02x-%02x",
		(uint64(*mac)&0x0000000000FF)>>0,
		(uint64(*mac)&0x00000000FF00)>>8,
		(uint64(*mac)&0x000000FF0000)>>16,
		(uint64(*mac)&0x0000FF000000)>>24,
		(uint64(*mac)&0x00FF00000000)>>32,
		(uint64(*mac)&0xFF0000000000)>>40,
	)), nil
}

type AutonomousSystem struct {
	Number uint32 `json:"number"`
}

type ECSSourceOrDest struct {
	Address          string `json:"address,omitempty"`
	Bytes            uint64 `json:"bytes,omitempty"`
	Domain           string `json:"domain,omitempty"`
	IP               string `json:"ip,omitempty"`
	MAC              string `json:"mac,omitempty"`
	Packets          uint64 `json:"packets,omitempty"`
	Port             uint32 `json:"port,omitempty"`
	RegisteredDomain string `json:"registered_domain,omitempty"`
	//Subdomain        string            `json:"subdomain,omitempty"`
	TopLevelDomain   string            `json:"top_level_domain,omitempty"`
	AutonomousSystem *AutonomousSystem `json:"autonomous_system,omitempty"`
}
