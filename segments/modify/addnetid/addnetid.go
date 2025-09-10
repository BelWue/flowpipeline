package addnetid

import (
	"encoding/csv"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/bwNetFlow/ip_prefix_trie"
	"github.com/rs/zerolog/log"
)

// The `addnetid` segment can add a network ID to flows according to the IP prefix
// the flow is matched to. These prefixes are sourced from a simple csv file
// consisting of lines in the format `ip prefix,id`. For example:
//
// ```csv
// 192.168.88.0/25,1
// 192.168.88.128/25,experiment-1
// 2001:db8:1::/48,1
// 2001:db8:2::/48,experiment-1
// ```
//
// Which IP address is matched against this database is determined by the
// RemoteAddress field of the flow. If this is unset, the flow is forwarded
// untouched. To set this field, see the `remoteaddress` segment. If matchboth is
// set to true, this segment will not try to establish the remote address and
// instead check both, source and destination address, in this order.
//
// If the useintids option is set to true the default is false all ids mus be valid integers
// If this mode is used the id will be written to the Src/DstId field of the enriched flow.
// If not it will be written to the Src/DstIdString Field of the enriched flow.
//
// If `dropunmatched` is set to true no untouched flows will pass this segment,
// regardless of the reason for the flow being unmatched (absence of RemoteAddress
// field, actually no matching entry in database).
// if `enforceint` is set to true all networkids must be valid integers and will be
// written to the SrcId Field in the enriched flow.

type AddNetId struct {
	segments.BaseSegment
	FileName      string // required
	DropUnmatched bool   // optional, default is false, determines whether flows are dropped when no Cid is found
	MatchBoth     bool   // optional, default is false, determines whether src and dst addresses are matched separately and not according to remote addresses
	UseIntIds     bool   // optional, default is true, enforce network ids to be valid unsigned 32 bit integer

	trieV4 ip_prefix_trie.TrieNode
	trieV6 ip_prefix_trie.TrieNode
}

func (segment AddNetId) New(config map[string]string) segments.Segment {
	drop, err := strconv.ParseBool(config["dropunmatched"])
	if err != nil {
		log.Info().Msg("AddNetId: 'dropunmatched' set to default 'false'.")
	}
	both, err := strconv.ParseBool(config["matchboth"])
	if err != nil {
		log.Info().Msg("AddNetId: 'matchboth' set to default 'false'.")
	}
	enforce, err := strconv.ParseBool(config["useintids"])
	if err != nil {
		log.Info().Msg("AddNetId: 'useintids' set to default 'false'.")
	}
	if config["filename"] == "" {
		log.Error().Msg("AddNetId: This segment requires a 'filename' parameter.")
		return nil
	}

	return &AddNetId{
		FileName:      config["filename"],
		DropUnmatched: drop,
		MatchBoth:     both,
		UseIntIds:     enforce,
	}
}

func (segment *AddNetId) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	segment.readPrefixList()
	for msg := range segment.In {
		var laddress net.IP
		if !segment.MatchBoth {
			switch {
			case msg.RemoteAddr == 1: // 1 indicates SrcAddr is the RemoteAddr
				laddress = msg.DstAddr // we want the LocalAddr tho
			case msg.RemoteAddr == 2: // 2 indicates DstAddr is the RemoteAddr
				laddress = msg.SrcAddr // we want the LocalAddr tho
			default:
				if !segment.DropUnmatched {
					segment.Out <- msg
				}
				continue
			}

			// prepare matching the address into a prefix and its associated CID
			if laddress.To4() == nil {
				if segment.UseIntIds {
					retId, _ := segment.trieV6.Lookup(laddress).(int64)
					msg.NetId = uint32(retId)
				} else {
					retId, _ := segment.trieV6.Lookup(laddress).(string)
					msg.NetIdString = retId
				}

			} else {
				if segment.UseIntIds {
					retId, _ := segment.trieV4.Lookup(laddress).(int64)
					msg.NetId = uint32(retId)
				} else {
					retId, _ := segment.trieV4.Lookup(laddress).(string)
					msg.NetIdString = retId
				}
			}
			if segment.DropUnmatched && msg.Cid == 0 {
				continue
			}
		} else {
			if net.IP(msg.SrcAddr).To4() == nil {
				if segment.UseIntIds {
					retId, _ := segment.trieV6.Lookup(msg.SrcAddr).(int64)
					msg.SrcId = uint32(retId)
				} else {
					retId, _ := segment.trieV6.Lookup(msg.SrcAddr).(string)
					msg.SrcIdString = retId
				}
			} else {
				if segment.UseIntIds {
					retId, _ := segment.trieV4.Lookup(msg.SrcAddr).(int64)
					msg.SrcId = uint32(retId)
				} else {
					retId, _ := segment.trieV4.Lookup(msg.SrcAddr).(string)
					msg.SrcIdString = retId
				}
			}
			if net.IP(msg.DstAddr).To4() == nil {
				if segment.UseIntIds {
					retId, _ := segment.trieV6.Lookup(msg.DstAddr).(int64)
					msg.DstId = uint32(retId)
				} else {
					retId, _ := segment.trieV6.Lookup(msg.DstAddr).(string)
					msg.DstIdString = retId
				}
			} else {
				if segment.UseIntIds {
					retId, _ := segment.trieV4.Lookup(msg.DstAddr).(int64)
					msg.DstId = uint32(retId)
				} else {
					retId, _ := segment.trieV4.Lookup(msg.DstAddr).(string)
					msg.DstIdString = retId
				}
			}
			if segment.UseIntIds {
				if msg.SrcId == 0 && msg.DstId != 0 {
					msg.NetId = msg.DstId
				} else if msg.DstId == 0 && msg.SrcId != 0 {
					msg.NetId = msg.SrcId
				} else if msg.DstId == 0 && msg.SrcId == 0 {
					if segment.DropUnmatched {
						continue
					}
				}
			} else {
				if msg.SrcIdString == "" && msg.DstIdString != "" {
					msg.NetIdString = msg.DstIdString
				} else if msg.DstIdString == "" && msg.SrcIdString != "" {
					msg.NetIdString = msg.SrcIdString
				} else if msg.DstIdString == "" && msg.SrcIdString == "" {
					continue
				}
			}
		}
		segment.Out <- msg
	}
}

func (segment *AddNetId) readPrefixList() {
	f, err := os.Open(segments.ContainerVolumePrefix + segment.FileName)
	if err != nil {
		log.Error().Err(err).Msg("AddNetId: Could not open prefix list: ")
		return
	}
	defer f.Close()

	csvr := csv.NewReader(f)
	var count int
	for {
		row, err := csvr.Read()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Warn().Err(err).Msg("AddNetId: Encountered non-CSV line in prefix list: ")
				continue
			}
		}
		var netid interface{}
		if segment.UseIntIds {
			nid, err := strconv.ParseInt(row[1], 10, 32)
			if err != nil {
				log.Warn().Err(err).Msg("AddNetId: Encountered non-integer customer id: ")
				continue
			} else {
				netid = nid
			}
		} else {
			netid = row[1]
		}

		// copied from net.IP module to detect v4/v6
		var added bool
		for i := 0; i < len(row[0]); i++ {
			switch row[0][i] {
			case '.':
				segment.trieV4.Insert(netid, []string{row[0]})
				added = true
			case ':':
				segment.trieV6.Insert(netid, []string{row[0]})
				added = true
			}
			if added {
				count += 1
				break
			}
		}
	}
	log.Info().Msgf("AddNetId: Read prefix list with %d prefixes.", count)
}

func init() {
	segment := &AddNetId{}
	segments.RegisterSegment("addnetid", segment)
}
