// Enriches any passing flow message with a customer id field based on a CIDR
// match. Requires a remote address to be marked in any given flow message by
// default, but lookups can be done for both source and destination address
// separately using the matchboth parameter. The result is written to the Cid
// field of any flow. If matchboth is set, SrcCid and DstCid will contain the
// results, with Cid containing an additional copy if just one address had a
// match.
package addcid

import (
	"encoding/csv"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/bwNetFlow/ip_prefix_trie"
)

type AddCid struct {
	segments.BaseSegment
	FileName      string // required
	DropUnmatched bool   // optional, default is false, determines whether flows are dropped when no Cid is found
	MatchBoth     bool   // optional, default is false, determines whether src and dst addresses are matched seperately and not according to remote addresses

	trieV4 ip_prefix_trie.TrieNode
	trieV6 ip_prefix_trie.TrieNode
}

func (segment AddCid) New(config map[string]string) segments.Segment {
	drop, err := strconv.ParseBool(config["dropunmatched"])
	if err != nil {
		log.Info().Msg("AddCid: 'dropunmatched' set to default 'false'.")
	}
	both, err := strconv.ParseBool(config["matchboth"])
	if err != nil {
		log.Info().Msg("AddCid: 'matchboth' set to default 'false'.")
	}
	if config["filename"] == "" {
		log.Error().Msg("AddCid: This segment requires a 'filename' parameter.")
		return nil
	}

	return &AddCid{
		FileName:      config["filename"],
		DropUnmatched: drop,
		MatchBoth:     both,
	}
}

func (segment *AddCid) Run(wg *sync.WaitGroup) {
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
				retCid, _ := segment.trieV6.Lookup(laddress).(int64) // try to get a CID
				msg.Cid = uint32(retCid)
			} else {
				retCid, _ := segment.trieV4.Lookup(laddress).(int64) // try to get a CID
				msg.Cid = uint32(retCid)
			}
			if segment.DropUnmatched && msg.Cid == 0 {
				continue
			}
		} else {
			if net.IP(msg.SrcAddr).To4() == nil {
				retCid, _ := segment.trieV6.Lookup(msg.SrcAddr).(int64) // try to get a CID
				msg.SrcCid = uint32(retCid)
			} else {
				retCid, _ := segment.trieV4.Lookup(msg.SrcAddr).(int64) // try to get a CID
				msg.SrcCid = uint32(retCid)
			}
			if net.IP(msg.DstAddr).To4() == nil {
				retCid, _ := segment.trieV6.Lookup(msg.DstAddr).(int64) // try to get a CID
				msg.DstCid = uint32(retCid)
			} else {
				retCid, _ := segment.trieV4.Lookup(msg.DstAddr).(int64) // try to get a CID
				msg.DstCid = uint32(retCid)
			}
			if msg.SrcCid == 0 && msg.DstCid != 0 {
				msg.Cid = msg.DstCid
			} else if msg.DstCid == 0 && msg.SrcCid != 0 {
				msg.Cid = msg.SrcCid
			}
		}
		segment.Out <- msg
	}
}

func (segment *AddCid) readPrefixList() {
	f, err := os.Open(segments.ContainerVolumePrefix + segment.FileName)
	if err != nil {
		log.Error().Err(err).Msg("AddCid: Could not open prefix list: ")
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
				log.Warn().Err(err).Msg("AddCid: Encountered non-CSV line in prefix list: ")
				continue
			}
		}

		cid, err := strconv.ParseInt(row[1], 10, 32)
		if err != nil {
			log.Warn().Err(err).Msg("AddCid: Encountered non-integer customer id: ")
			continue
		}

		// copied from net.IP module to detect v4/v6
		var added bool
		for i := 0; i < len(row[0]); i++ {
			switch row[0][i] {
			case '.':
				segment.trieV4.Insert(cid, []string{row[0]})
				added = true
			case ':':
				segment.trieV6.Insert(cid, []string{row[0]})
				added = true
			}
			if added {
				count += 1
				break
			}
		}
	}
	log.Info().Msgf("AddCid: Read prefix list with %d prefixes.", count)
}

func init() {
	segment := &AddCid{}
	segments.RegisterSegment("addcid", segment)
}
