// Determines the remote address of flows based on different criteria.
//
// This segment basically runs flows through a switch case directive using its
// only config parameter, 'policy'. Some policies unlock additional parameters.
//
// 'cidr' matches first source address and then destination address against the
// prefixed provided in the CSV file using the config parameter 'filename'.
// When a match to a non-zero value occurs, the RemoteAddress indicator is set
// accordingly. Accordingly means that a non-zero value is assumed to be a
// customer ID as used by the 'AddCid' segment, i.e. the customer-address
// (whether it's source or destination) will be considered local. Optionally,
// unmatched flows can be dropped using 'dropunmatched'.
//
// 'border' assumes flows are exported on border interfaces: If a flow's
// direction is 'ingress' on such an interface, its remote address is the
// source address of the flow, whereas the local address of the flow would be
// its destination address inside our network. The same logic applies vice
// versa: 'egress' flows have a remote destination address.
//
// 'user' assumes flows are exported on user interfaces: If a flow's
// direction is 'ingress' on such an interface, its remote address is the
// destination address of the flow, whereas the local address of the flow would be
// its destination address inside our user's network. The same logic applies
// vice versa: 'egress' flows have a remote source address.
//
// 'clear' assumes flows are exported whereever, and thus all remote address
// info is cleared in this case.
package remoteaddress

import (
	"encoding/csv"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/bwNetFlow/ip_prefix_trie"
)

type RemoteAddress struct {
	segments.BaseSegment
	Policy        string // required, 'cidr', 'border', 'user' and 'clear' are available options, see above
	FileName      string // optional, required if policy is set to 'cidr', default is empty
	DropUnmatched bool   // optional, default is false, relevant to 'cidr' only, determines what to do with unmatched flows

	trieV4 ip_prefix_trie.TrieNode
	trieV6 ip_prefix_trie.TrieNode
}

func (segment RemoteAddress) New(config map[string]string) segments.Segment {
	if !(config["policy"] == "cidr" || config["policy"] == "border" || config["policy"] == "user" || config["policy"] == "clear") {
		log.Error().Msg("RemoteAddress: The 'policy' parameter is required to be one of 'cidr', 'border', 'user', or 'clear'.")
		return nil
	}
	drop, err := strconv.ParseBool(config["dropunmatched"])
	if err != nil {
		log.Info().Msg("RemoteAddress: 'dropunmatched' set to default 'false'.")
	}
	if config["policy"] == "cidr" && config["filename"] == "" {
		log.Error().Msg("RemoteAddress: This segment requires a 'filename' parameter.")
		return nil
	}
	return &RemoteAddress{
		Policy:        config["policy"],
		FileName:      config["filename"],
		DropUnmatched: drop,
	}
}

func (segment *RemoteAddress) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	switch segment.Policy {
	case "cidr":
		segment.readPrefixList()
		for msg := range segment.In {
			var ret int64
			for i, addr := range []net.IP{msg.SrcAddr, msg.DstAddr} {
				if addr.To4() == nil {
					ret, _ = segment.trieV6.Lookup(addr).(int64)
				} else {
					ret, _ = segment.trieV4.Lookup(addr).(int64)
				}
				if ret != 0 {
					msg.RemoteAddr = pb.EnrichedFlow_RemoteAddrType(i + 1)
					break
				} else if segment.DropUnmatched {
					continue
				}
			}
			segment.Out <- msg
		}
	case "border":
		for msg := range segment.In {
			switch {
			case msg.FlowDirection == 0: // flow is ingress on border interface
				msg.RemoteAddr = 1 // thus, RemoteAddr should indicate SrcAddr
			case msg.FlowDirection == 1: // flow is egress on border interface
				msg.RemoteAddr = 2 // thus, RemoteAddr should indicate DstAddr
			}
			segment.Out <- msg
		}
	case "user":
		for msg := range segment.In {
			switch {
			case msg.FlowDirection == 0: // flow is ingress on user interface
				msg.RemoteAddr = 2 // thus, RemoteAddr should indicate DstAddr
			case msg.FlowDirection == 1: // flow is egress on user interface
				msg.RemoteAddr = 1 // thus, RemoteAddr should indicate SrcAddr
			}
			segment.Out <- msg
		}
	case "clear":
		for msg := range segment.In {
			msg.RemoteAddr = 0 // reset previous info, we can't tell in a mixed env
			segment.Out <- msg
		}
	}
}

func (segment *RemoteAddress) readPrefixList() {
	f, err := os.Open(segments.ContainerVolumePrefix + segment.FileName)
	if err != nil {
		log.Error().Err(err).Msg("RemoteAddress: Could not open prefix list: ")
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
				log.Warn().Err(err).Msg("RemoteAddress: Encountered non-CSV line in prefix list: ")
				continue
			}
		}

		cid, err := strconv.ParseInt(row[1], 10, 32)
		if err != nil {
			log.Warn().Err(err).Msg("RemoteAddress: Encountered non-integer customer id: ")
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
	log.Info().Msgf("RemoteAddress: Read prefix list with %d prefixes.", count)
}

func init() {
	segment := &RemoteAddress{}
	segments.RegisterSegment("remoteaddress", segment)
}
