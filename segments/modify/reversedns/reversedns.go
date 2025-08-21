// The `reversedns` segment looks up DNS PTR records for Src, Dst, Sampler and
// NextHopAddr and adds them to our flows. The results are also written to a internal
// cache which works well for ad-hoc usage, but it's recommended to use an actual
// caching resolver in real deployment scenarios. The refresh interval setting pertains
// to the internal cache only.
package reversedns

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/dnscache"
)

type ReverseDns struct {
	Cache           bool   // optional, default is true, disable to use a caching resolver directly
	RefreshInterval string // optional, default is 5m, set another duration for cache refreshes

	resolver *dnscache.Resolver
	segments.BaseSegment
}

func (segment ReverseDns) New(config map[string]string) segments.Segment {
	newsegment := &ReverseDns{
		resolver: &dnscache.Resolver{},
	}

	var cache bool = true
	if config["cache"] != "" {
		var err error
		if cache, err = strconv.ParseBool(config["cache"]); err == nil {
			log.Error().Msg("ReverseDns: Invalid 'cache' parameter.")
			return nil
		}
	}

	if cache {
		refresh := "5m"
		if config["refreshinterval"] != "" {
			refresh = config["refreshinterval"]
		}
		duration, err := time.ParseDuration(refresh)
		if err != nil {
			log.Error().Msg("ReverseDns: Invalid 'refreshinterval' parameter.")
			return nil
		}
		go func() {
			t := time.NewTicker(duration)
			defer t.Stop()
			for range t.C {
				newsegment.resolver.Refresh(true)
			}
		}()
	}
	return newsegment
}

func (segment *ReverseDns) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		hostnames, err := segment.resolver.LookupAddr(context.Background(), msg.SrcAddrObj().String())
		if err == nil && len(hostnames) > 0 {
			msg.SrcHostName = hostnames[0]
		}
		hostnames, err = segment.resolver.LookupAddr(context.Background(), msg.DstAddrObj().String())
		if err == nil && len(hostnames) > 0 {
			msg.DstHostName = hostnames[0]
		}
		hostnames, err = segment.resolver.LookupAddr(context.Background(), msg.NextHopObj().String())
		if err == nil && len(hostnames) > 0 {
			msg.NextHopHostName = hostnames[0]
		}
		hostnames, err = segment.resolver.LookupAddr(context.Background(), msg.SamplerAddressObj().String())
		if err == nil && len(hostnames) > 0 {
			msg.SamplerHostName = hostnames[0]
		}
		// TODO: Add support for looking up AS names as well. This
		// requires some function such as:
		//
		// asnames, err := segment.resolver.LookupTxt(context.Background(), fmt.Sprintf("AS%d.asn.cymru.com", msg.SrcAS))
		//
		// Which would be trivial to add here, if the cache of our
		// choice provided support for TXT record lookups. This will
		// have to be added upstream.
		segment.Out <- msg
	}
}

func init() {
	segment := &ReverseDns{}
	segments.RegisterSegment("reversedns", segment)
}
