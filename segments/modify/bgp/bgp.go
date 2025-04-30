// Enriches flows with infos from BGP.
package bgp

import (
	"net"
	"os"
	"slices"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/bgp_routeinfo/routeinfo"
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"gopkg.in/yaml.v2"
)

type Bgp struct {
	segments.BaseSegment
	FileName        string // required
	FallbackRouter  string // optional, default is "" (i.e., none or disabled), this will determine the BGP session that is used when SamplerAddress has no corresponding session
	UseFallbackOnly bool   // optional, default is false, this will disable looking for SamplerAddress BGP sessions
	RouterASN       uint32 // ASN of the local router

	routeInfoServer routeinfo.RouteInfoServer
}

func (segment Bgp) New(config map[string]string) segments.Segment {
	rsconfig, err := os.ReadFile(config["filename"])
	if err != nil {
		log.Error().Err(err).Msg("Bgp: Error reading BGP session config file: ")
		return nil
	}
	var rs routeinfo.RouteInfoServer
	err = yaml.Unmarshal(rsconfig, &rs)
	if err != nil {
		log.Error().Err(err).Msg("Bgp: Error parsing BGP session configuration YAML: ")
		return nil
	}

	var routerASN uint32
	var raw map[string]interface{}
	if err := yaml.Unmarshal(rsconfig, &raw); err == nil {
		if val, ok := raw["asn"]; ok {
			switch v := val.(type) {
			case int:
				routerASN = uint32(v)
			case float64:
				routerASN = uint32(v)
			case string:
				asn, err := strconv.ParseUint(v, 10, 32)
				if err == nil {
					routerASN = uint32(asn)
				} else {
					log.Warn().Str("asn", v).Msg("Bgp: Invalid ASN format in YAML; ignoring")
				}
			}
		}
	}

	if fallback, present := config["fallbackrouter"]; present {
		if _, ok := rs.Routers[fallback]; !ok {
			log.Error().Msgf("Bgp: No fallback router named '%s' has been configured.", fallback)
			return nil
		}
	}

	fallbackonly, err := strconv.ParseBool(config["usefallbackonly"])
	if err != nil {
		log.Info().Msg("Bgp: 'usefallbackonly' set to default 'false'.")
	}
	if fallbackonly && config["fallbackrouter"] == "" {
		log.Error().Msgf("Bgp: Forcing fallback requires a fallbackrouter parameter.")
		return nil
	}

	newSegment := &Bgp{
		FileName:        config["filename"],
		FallbackRouter:  config["fallbackrouter"],
		UseFallbackOnly: fallbackonly,
		RouterASN:       routerASN,
		routeInfoServer: rs,
	}
	return newSegment
}

func (segment *Bgp) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	segment.routeInfoServer.Init()
	defer func() {
		segment.routeInfoServer.Stop()
	}()

	for msg := range segment.In {
		// The following conversions to String are stupid, but it is
		// what gobgp requires at the end of this call hierarchy.
		var dstRouteInfos []routeinfo.RouteInfo
		var srcRouteInfos []routeinfo.RouteInfo
		var srcAsPath []uint32
		var dstAsPath []uint32
		if segment.UseFallbackOnly {
			dstRouteInfos = segment.routeInfoServer.Routers[segment.FallbackRouter].Lookup(msg.DstAddrObj().String())
			srcRouteInfos = segment.routeInfoServer.Routers[segment.FallbackRouter].Lookup(msg.SrcAddrObj().String())
		} else {
			if router, ok := segment.routeInfoServer.Routers[msg.SamplerAddressObj().String()]; ok {
				dstRouteInfos = router.Lookup(msg.DstAddrObj().String())
				srcRouteInfos = router.Lookup(msg.SrcAddrObj().String())
			} else if segment.FallbackRouter != "" {
				dstRouteInfos = segment.routeInfoServer.Routers[segment.FallbackRouter].Lookup(msg.DstAddrObj().String())
				srcRouteInfos = segment.routeInfoServer.Routers[segment.FallbackRouter].Lookup(msg.SrcAddrObj().String())
			} else {
				segment.Out <- msg
				continue
			}
		}

		for _, path := range srcRouteInfos {
			if !path.Best || len(path.AsPath) == 0 {
				continue
			}
			slices.Reverse(path.AsPath)
			msg.AsPath = path.AsPath
			srcAsPath = path.AsPath
			switch path.Validation {
			case routeinfo.Valid:
				msg.ValidationStatus = pb.EnrichedFlow_Valid
			case routeinfo.NotFound:
				msg.ValidationStatus = pb.EnrichedFlow_NotFound
			case routeinfo.Invalid:
				msg.ValidationStatus = pb.EnrichedFlow_Invalid
			default:
				msg.ValidationStatus = pb.EnrichedFlow_Unknown
			}
			break
		}
		if segment.RouterASN != 0 {
			msg.AsPath = append(msg.AsPath, uint32(segment.RouterASN))
			srcAsPath = append(srcAsPath, uint32(segment.RouterASN))
		}

		for _, path := range dstRouteInfos {
			if !path.Best || len(path.AsPath) == 0 {
				continue
			}
			dstAsPath = append([]uint32{segment.RouterASN}, path.AsPath...)
			msg.AsPath = append(msg.AsPath, dstAsPath...)
			msg.Med = path.Med
			msg.LocalPref = path.LocalPref
			switch path.Validation {
			case routeinfo.Valid:
				msg.ValidationStatus = pb.EnrichedFlow_Valid
			case routeinfo.NotFound:
				msg.ValidationStatus = pb.EnrichedFlow_NotFound
			case routeinfo.Invalid:
				msg.ValidationStatus = pb.EnrichedFlow_Invalid
			default:
				msg.ValidationStatus = pb.EnrichedFlow_Unknown
			}
			// for router exported netflow, the following are likely overwriting their own annotations
			if len(path.AsPath) > 0 {
				msg.DstAs = path.AsPath[len(path.AsPath)-1]
				msg.NextHopAs = path.AsPath[0]
			}
			if nh := net.ParseIP(path.NextHop); nh != nil {
				msg.NextHop = nh
			}
			break
		}

		msg.SrcAsPath = srcAsPath
		msg.DstAsPath = dstAsPath
		segment.Out <- msg
	}
}

func init() {
	segment := &Bgp{}
	segments.RegisterSegment("bgp", segment)
}
