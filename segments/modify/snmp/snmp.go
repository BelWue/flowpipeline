// The `snmp` segment annotates flows with interface information learned
// directly from routers using SNMP. This is a potentially perfomance impacting
// segment and has to be configured carefully.
//
// In principle, this segment tries to fetch a SNMP OID datapoint from the address
// in SamplerAddress, which corresponds to a router on normal flow-exporter
// generated flows. The fields used to query this router are SrcIf and DstIf, i.e.
// the interface IDs which are part of the flow. If successfull, a flow will have
// the fields `{Src,Dst}IfName`, `{Src,Dst}IfDesc`, and `{Src,Dst}IfSpeed`
// populated. In order to not to overload the router and to introduce delays, this
// segment will:
//
//   - not wait for a SNMP query to return, instead it will leave the flow as it was
//     before sending it to the next segment (i.e. the first one on a given
//     interface will always remain untouched)
//   - add any interface's data to a cache, which will be used to enrich the
//     next flow using that same interface
//   - clear the cache value after 1 hour has elapsed, resulting in another flow
//     without these annotations at that time
//
// These rules are applied for source and destination interfaces separately.
//
// The paramters to this segment specify the SNMPv2 community as well as the
// connection limit employed by this segment. The latter is again to not overload
// the routers SNMPd. Lastly, the regex parameter can be used to limit the
// `IfDesc` annotations to a certain part of the actual interface description.
// For instance, descriptions follow the format `customerid - blablalba`, the
// regex `(.*) -.*` would grab just that customer ID to put into the `IfDesc`
// fields. Also see the full examples linked below.
//
// Roadmap:
// * cache timeout should be configurable
package snmp

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/alouca/gosnmp"
	cache "github.com/patrickmn/go-cache"
)

var (
	oidBase = ".1.3.6.1.2.1.31.1.1.1.%d.%d"
	oidExts = map[string]uint8{"name": 1, "speed": 15, "desc": 18}
)

type Snmp struct {
	segments.BaseSegment
	Community string // optional, default is 'public'
	Regex     string // optional, default matches all, can be used to extract content from descriptions, see examples/enricher
	ConnLimit uint64 // optional, default is 16

	compiledRegex      *regexp.Regexp
	snmpCache          *cache.Cache
	connLimitSemaphore chan struct{}
}

func (segment Snmp) New(config map[string]string) segments.Segment {
	var connLimit uint64 = 16
	if config["connlimit"] != "" {
		if parsedConnLimit, err := strconv.ParseUint(config["connlimit"], 10, 32); err == nil {
			connLimit = parsedConnLimit
			if connLimit == 0 {
				log.Error().Msg("SNMP: Limiting connections to 0 will not work. Remove this segment or use a higher value (recommendation >= 16).")
				return nil
			}
		} else {
			log.Error().Msg("SNMP: Could not parse 'connlimit' parameter, using default 16.")
		}
	} else {
		log.Info().Msg("SNMP: 'connlimit' set to default '16'.")
	}

	var community string = "public"
	if config["community"] != "" {
		community = config["community"]
	} else {
		log.Info().Msg("SNMP: 'community' set to default 'public'.")
	}
	var regex string = "^(.*)$"
	if config["regex"] != "" {
		regex = config["regex"]
	} else {
		log.Info().Msg("SNMP: 'regex' set to default '^(.*)$'.")
	}
	compiledRegex, err := regexp.Compile(regex)
	if err != nil {
		log.Error().Err(err).Msg("SNMP: Configuration error, regex does not compile: ")
		return nil
	}
	return &Snmp{
		Community:     community,
		Regex:         regex,
		ConnLimit:     connLimit,
		compiledRegex: compiledRegex,
	}
}

func (segment *Snmp) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	// init cache:			expiry       purge
	segment.snmpCache = cache.New(1*time.Hour, 1*time.Hour) // TODO: make configurable
	// init semaphore for connection limit
	segment.connLimitSemaphore = make(chan struct{}, segment.ConnLimit)

	for msg := range segment.In {
		router := net.IP(msg.SamplerAddress).String()
		// TODO: rename SrcIf and DstIf fields to match goflow InIf/OutIf
		if msg.InIf > 0 {
			msg.SrcIfName, msg.SrcIfDesc, msg.SrcIfSpeed = segment.fetchInterfaceData(router, msg.InIf)
			if msg.SrcIfDesc != "" {
				cleanDesc := segment.compiledRegex.FindStringSubmatch(msg.SrcIfDesc)
				if len(cleanDesc) > 1 {
					msg.SrcIfDesc = cleanDesc[1]
				}
			}
		}
		if msg.OutIf > 0 {
			msg.DstIfName, msg.DstIfDesc, msg.DstIfSpeed = segment.fetchInterfaceData(router, msg.OutIf)
			if msg.DstIfDesc != "" {
				cleanDesc := segment.compiledRegex.FindStringSubmatch(msg.DstIfDesc)
				if len(cleanDesc) > 1 {
					msg.DstIfDesc = cleanDesc[1]
				}
			}
		}
		segment.Out <- msg
	}
}

// Query a single SNMP datapoint. Supposedly a short-lived goroutine.
func (segment *Snmp) querySNMP(router string, iface uint32, key string) {
	defer func() {
		<-segment.connLimitSemaphore // release
	}()
	segment.connLimitSemaphore <- struct{}{} // acquire

	s, err := gosnmp.NewGoSNMP(router, segment.Community, gosnmp.Version2c, 1)
	if err != nil {
		log.Error().Err(err).Msg("SNMP: Connection Error")
		segment.snmpCache.Delete(fmt.Sprintf("%s-%d-%s", router, iface, key))
		return
	}

	var result *gosnmp.SnmpPacket
	oid := fmt.Sprintf(oidBase, oidExts[key], iface)
	resp, err := s.Get(oid)
	if err != nil {
		log.Warn().Err(err).Msgf("SNMP: Failed getting OID '%s' from %s.", oid, router)
		segment.snmpCache.Delete(fmt.Sprintf("%s-%d-%s", router, iface, key))
		return
	} else {
		result = resp
	}

	// parse and cache
	if len(result.Variables) == 1 {
		snmpvalue := resp.Variables[0].Value
		segment.snmpCache.Set(fmt.Sprintf("%s-%d-%s", router, iface, key), snmpvalue, cache.DefaultExpiration)
	} else {
		log.Warn().Msgf("SNMP: Bad response getting %s from %s. Error: %v", key, router, resp.Variables)
	}
}

// Fetch interface data from cache or from the live router. The latter is done
// async, so this method will return nils on the first call for any specific interface.
func (segment *Snmp) fetchInterfaceData(router string, iface uint32) (string, string, uint32) {
	var name, desc string
	var speed uint32
	for key := range oidExts {
		// if value in cache and cache content is not nil, i.e. marked as "being queried"
		if value, found := segment.snmpCache.Get(fmt.Sprintf("%s-%d-%s", router, iface, key)); found {
			if value == nil { // this occures if a goroutine is querying this interface
				return "", "", 0
			}
			switch key {
			case "name":
				name = value.(string)
			case "desc":
				desc = value.(string)
			case "speed":
				speed = uint32(value.(uint64))
			}
		} else {
			// mark as "being queried" by putting nil into the cache, so a future run will use the cached nil
			segment.snmpCache.Set(fmt.Sprintf("%s-%d-%s", router, iface, key), nil, cache.DefaultExpiration)
			// go query it
			go segment.querySNMP(router, iface, key)
		}
	}
	return name, desc, speed
}

// @Deprecated: Wrapper for old segment name. Gonna be removed
type SNMPInterface struct {
	Snmp
}

func (segment SNMPInterface) New(config map[string]string) segments.Segment {
	log.Warn().Msg("Using deprected segment 'snmpinterface'. Please use 'snmp' instead")
	return segment.Snmp.New(config)
}

func (segment *SNMPInterface) Run(wg *sync.WaitGroup) {
	log.Warn().Msg("Using deprected segment 'snmpinterface'. Please use 'snmp' instead")
	segment.Snmp.Run(wg)
}

func init() {
	segment := &Snmp{}
	segments.RegisterSegment("snmp", segment)

	deprecatedSegment := &SNMPInterface{}
	segments.RegisterSegment("snmpinterface", deprecatedSegment)
}
