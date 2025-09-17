# flowpipeline Configuration and User Guide

_This document was generated from '[meta/doc_generator/main.go](https://github.com/BelWue/flowpipeline/tree/master/meta/doc_generator/main.go)', based on commit '[22d84cd932be9141cde483d6472ee52bdf49c4b5](https://github.com/BelWue/flowpipeline/commit/22d84cd932be9141cde483d6472ee52bdf49c4b5)'._

Any flowpipeline is configured in a single yaml file which is either located in
the default `config.yml` or specified using the `-c` option when calling the
binary. The config file contains a single list of so-called segments, which
are processing flows in order. Flows represented by
[protobuf messages](https://github.com/bwNetFlow/protobuf/blob/master/flow-messages-enriched.proto)
within the pipeline.

Usually, the first segment is from the _input_ group, followed by any number of
different segments. Often, flowpipelines end with a segment from the _output_,
_print_, or _export_ groups. All segments, regardless from which group, accept and
forward their input from previous segment to their subsequent segment, i.e.
even input or output segments can be chained to one another or be placed in the
middle of a pipeline.

This overview is structures as follows:

<details>
<summary>Table of Contents</summary>

- [Alert Group](#alert-group)
  - [http](#http)
- [Analysis Group](#analysis-group)
  - [toptalkers_metrics](#toptalkers_metrics)
  - [Traffic_specific_toptalkers Group](#traffic_specific_toptalkers-group)
- [Controlflow Group](#controlflow-group)
  - [branch](#branch)
- [Dev Group](#dev-group)
  - [filegate](#filegate)
- [Filter Group](#filter-group)
  - [aggregate](#aggregate)
  - [drop](#drop)
  - [elephant](#elephant)
  - [flowfilter](#flowfilter)
- [Input Group](#input-group)
  - [bpf](#bpf)
  - [diskbuffer](#diskbuffer)
  - [goflow](#goflow)
  - [kafkaconsumer](#kafkaconsumer)
  - [packet](#packet)
  - [replay](#replay)
  - [stdin](#stdin)
- [Meta Group](#meta-group)
  - [Monitoring Group](#monitoring-group)
- [Modify Group](#modify-group)
  - [addcid](#addcid)
  - [addnetid](#addnetid)
  - [addrstrings](#addrstrings)
  - [anonymize](#anonymize)
  - [aslookup](#aslookup)
  - [bgp](#bgp)
  - [dropfields](#dropfields)
  - [geolocation](#geolocation)
  - [normalize](#normalize)
  - [protomap](#protomap)
  - [remoteaddress](#remoteaddress)
  - [reversedns](#reversedns)
  - [snmp](#snmp)
  - [sync_timestamps](#sync_timestamps)
- [Output Group](#output-group)
  - [clickhouse](#clickhouse)
  - [csv](#csv)
  - [influx](#influx)
  - [json](#json)
  - [kafkaproducer](#kafkaproducer)
  - [lumberjack](#lumberjack)
  - [Mongodb Group](#mongodb-group)
  - [prometheus](#prometheus)
  - [sqlite](#sqlite)
- [pass](#pass)
- [Print Group](#print-group)
  - [count](#count)
  - [printdots](#printdots)
  - [printflowdump](#printflowdump)
  - [toptalkers](#toptalkers)
- [Testing Group](#testing-group)
  - [generator](#generator)


</details>

## Available Segments

### Alert Group

Segments in this group alert external endpoints via different channels. They're usually
placed behind some form of filtering and include data from flows making it to this
segment in their notification.

#### http

_This segment is implemented in [http.go](https://github.com/BelWue/flowpipeline/tree/master/segments/alert/http/http.go)._

The `http` segment is currently a work in progress and is limited to sending
post requests with the full flow data to a single endpoint at the moment. The
roadmap includes various features such as different methods, builtin
conditional, limiting payload data, and multiple receivers.

<details>
<summary>Configuration options</summary>

* **Url** _string_

</details>

### Analysis Group

Segments in this group do higher level analysis on flow data. They usually export or
print results in some way, but might also filter given flows.

#### toptalkers_metrics

_This segment is implemented in [toptalkers_metrics.go](https://github.com/BelWue/flowpipeline/tree/master/segments/analysis/toptalkers_metrics/toptalkers_metrics.go)._

The `toptalkers_metrics` segment calculates statistics about traffic levels
per IP address and exports them in OpenMetrics format via HTTP.

Traffic is counted in bits per second and packets per second, categorized into
forwarded and dropped traffic. By default, only the destination IP addresses
are accounted, but the configuration allows using the source IP address or
both addresses. For the latter, a flows number of bytes and packets are
counted for both addresses. `connection` is used to look a specific combinations
of "source -> target".

Thresholds for bits per second or packets per second can be configured. Only
metrics for addresses that exceeded this threshold during the last window size
are exported. This can be used for detection of unusual or unwanted traffic
levels. This can also be used as a flow filter: While the average traffic for
an address is above threshold, flows are passed, other flows are dropped.

The averages are calculated with a sliding window. The window size (in number
of buckets) and the bucket duration can be configured. By default, it uses
60 buckets of 1 second each (1 minute of sliding window). Optionally, the
window size for the exported metrics calculation and for the threshold check
can be configured differently.

The parameter "traffictype" is passed as OpenMetrics label, so this segment
can be used multiple times in one pipeline without metrics getting mixed up.

#### Traffic_specific_toptalkers Group

_No group documentation found._

### Controlflow Group

Segments in this group have the ability to change the sequence of segments any given
flow traverses.

#### branch

_This segment is implemented in [branch.go](https://github.com/BelWue/flowpipeline/tree/master/segments/controlflow/branch/branch.go)._

The `branch` segment is used to select the further progression of the pipeline
between to branches. To this end, it uses additional syntax that other segments
do not have access to, namely the `if`, `then` and `else` keys which can
contain lists of segments that constitute embedded pipelines.

The any of these three keys may be empty and they are by default. The `if`
segments receive the flows entering the `branch` segment unconditionally. If
the segments in `if` proceed any flow from the input all the way to the end of
the `if` segments, this flow will be moved on to the `then` segments. If flows
are dropped at any point within the `if` segments, they will be moved on to the
`else` branch immediately, shortcutting the traversal of the `if` segments. Any
edits made to flows during the `if` segments will be persisted in either
branch, `then` and `else`, as well as after the flows passed from the `branch`
segment into consecutive segments. Dropping flows behaves regularly in both
branches, but note that flows can not be dropped within the `if` branch
segments, as this is taken as a cue to move them into the `else` branch.
The `bypass-messages` flag can be used to forward all incoming messages to the
next segment, regardles of them being droped or forwarded inside the `if` or `else`
branch.

If any of these three lists of segments (or subpipelines) is empty, the
`branch` segment will behave as if this subpipeline consisted of a single
`pass` segment.

Instead of a minimal example, the following more elaborate one highlights all
TCP flows while printing to standard output and keeps only these highlighted
ones in a sqlite export:

```yaml
- segment: branch
  if:
  - segment: flowfilter
    config:
      filter: proto tcp
  - segment: elephant
  then:
  - segment: printflowdump
    config:
      highlight: 1
  else:
  - segment: printflowdump
  - segment: drop

- segment: sqlite
  config:
    filename: tcponly.sqlite
```

### Dev Group

_No group documentation found._

#### filegate

_This segment is implemented in [filegate.go](https://github.com/BelWue/flowpipeline/tree/master/segments/dev/filegate/filegate.go)._

Serves as a template for new segments and forwards flows, otherwise does
nothing.

### Filter Group

Segments in this group all drop flows, i.e. remove them from the pipeline from this
segment on. Fields in individual flows are never modified, only used as criteria.

#### aggregate

_This segment is implemented in [aggregate.go](https://github.com/BelWue/flowpipeline/tree/master/segments/filter/aggregate/aggregate.go)._

_No segment documentation found._

#### drop

_This segment is implemented in [drop.go](https://github.com/BelWue/flowpipeline/tree/master/segments/filter/drop/drop.go)._

The `drop` segment is used to drain a pipeline, effectively starting a new
pipeline after it. In conjunction with `skip`, this can act as a `flowfilter`.

#### elephant

_This segment is implemented in [elephant.go](https://github.com/BelWue/flowpipeline/tree/master/segments/filter/elephant/elephant.go)._

The `elephant` segment uses a configurable sliding window to determine flow
statistics at runtime and filter out unremarkable flows from the pipeline. This
segment can be configured to look at different aspects of single flows, i.e.
either the plain byte/packet counts or the average of those per second with
regard to flow duration. By default, it drops the lower 99% of flows with
regard to the configured aspect and does not use exact percentile matching,
instead relying on the much faster P-square estimation. For quick ad-hoc usage,
it can be useful to adjust the window size (in seconds).
The ramp up time defults to 0 (disabled), but can be configured to wait for analyzing
flows. All flows within this Timerange are dropped after the start of the pipeline.

<details>
<summary>Configuration options</summary>

* **Aspect** _string_
* **Percentile** _float64_
* **Exact** _bool_: TODO: add option to get bottom percent?

* **Window** _int_
* **RampupTime** _int_

</details>

#### flowfilter

_This segment is implemented in [flowfilter.go](https://github.com/BelWue/flowpipeline/tree/master/segments/filter/flowfilter/flowfilter.go)._

The `flowfilter` segment uses [flowfilter syntax](https://github.com/BelWue/flowfilter)
to drop flows based on the evaluation value of the provided filter conditional against
any flow passing through this segment.

<details>
<summary>Configuration options</summary>

* **Filter** _string_

</details>

### Input Group

Segments in this group import or collect flows and provide them to all following
segments. As all other segments do, these still forward incoming flows to the next
segment, i.e. multiple input segments can be used in sequence to add to a single
pipeline.

If you are connected behind [BelWü](https://www.belwue.de) contact one of our developers
or the [bwNet project](bwnet.belwue.de) for access to our central Kafka cluster. We'll
gladly create a topic for your university if it doesn't exist already and provide you
with the proper configuration to use this segment.

If the person responsible at university agrees, we will pre-fill this topic with flows
sent or received by your university as seen on our upstream border interfaces. This can
be limited according to the data protection requirements set forth by the universities.

#### bpf

_This segment is implemented in [bpf.go](https://github.com/BelWue/flowpipeline/tree/master/segments/input/bpf/bpf.go)._

**This segment is available only on Linux.**

The `bpf` segment sources packet header data from a local interface and uses
this data to run a Netflow-style cache before emitting flow data to the
pipeline.

This however has some caveats:
* using this segment requires root privileges to place BPF code in kernel
* the default kernel perf buffer size of 64kB should be sufficient on recent
  CPUs for up to 1Gbit/s of traffic, but requires tweaks in some scenarios
* the linux kernel version must be reasonably recent (probably 4.18+, certainly 5+)

Roadmap:
* allow hardware offloading to be configured
* implement sampling

<details>
<summary>Configuration options</summary>

* **Device** _string_
* **ActiveTimeout** _string_
* **InactiveTimeout** _string_
* **BufferSize** _int_

</details>

#### diskbuffer

_This segment is implemented in [diskbuffer.go](https://github.com/BelWue/flowpipeline/tree/master/segments/input/diskbuffer/diskbuffer.go)._

The `diskbuffer` segment buffers flows in memory and on-demand on disk.
Writing to disk is done in the JSON representation of the flows, compressed using
`zstd`. The flows are written to disk, when the MemoryBuffer reaches the percentual
fill level HighMemoryMark, until the LowMemoryMark is reached again. Files are read
from disk if the fill level reaches ReadingMemoryMark. The maximum file size and the
maximum size on disk are configurable via the `filesize` and `maxcachesize` parameter.

<details>
<summary>Configuration options</summary>

* **BatchSize** _int_
* **FileSize** _uint64_
* **BufferDir** _string_
* **HighMemoryMark** _int_
* **LowMemoryMark** _int_
* **ReadingMemoryMark** _int_
* **MaxCacheSize** _uint64_
* **Capacity** _int_

</details>

#### goflow

_This segment is implemented in [goflow.go](https://github.com/BelWue/flowpipeline/tree/master/segments/input/goflow/goflow.go)._

The `goflow` segment provides a convenient interface for
[goflow2](https://github.com/netsampler/goflow2) right from flowpipeline
config. It behaves closely like a regular goflow2 instance but provides flows
directly in our extended format (which has been based on goflow's from the
beginning of this project).

This flow collector needs to receive input from any IPFIX/Netflow/sFlow
exporters, for instance your network devices.

<details>
<summary>Configuration options</summary>

* **Workers** _uint64_
* **Blocking** _bool_
* **QueueSize** _int_
* **NumSockets** _int_

</details>

#### kafkaconsumer

_This segment is implemented in [kafkaconsumer.go](https://github.com/BelWue/flowpipeline/tree/master/segments/input/kafkaconsumer/kafkaconsumer.go)._

The `kafkaconsumer` segment consumes flows from a Kafka topic. This topic can
be created using the `kafkaproducer` module or using an external instance of
[goflow2](https://github.com/netsampler/goflow2).

This segment can be used in conjunction with the `kafkaproducer` segment to
enrich, reduce, or filter flows in transit between Kafka topics, or even sort
them into different Kafka topics. See the examples this particular usage.

The startat configuration sets whether to start at the newest or oldest
available flow (i.e. Kafka offset). It only takes effect if Kafka has no stored
state for this specific user/topic/consumergroup combination.

The supported group partion assignor balancing strategies can be set using a comma
separated list for `strategy`. Supported values are `sticky`, `roundrobin` and
`range`. Default is `sticky`.

<details>
<summary>Configuration options</summary>

* **Server** _string_
* **Topic** _string_
* **Group** _string_
* **User** _string_
* **Pass** _string_
* **Tls** _bool_
* **Auth** _bool_
* **StartAt** _string_
* **Legacy** _bool_
* **KafkaVersion** _string_

</details>

#### packet

_This segment is implemented in [packet.go](https://github.com/BelWue/flowpipeline/tree/master/segments/input/packet/packet.go)._

**This segment is available only on Linux.**
**This segment is available in the static binary release with some caveats in configuration.**

The `packet` segment sources packet header data from a local interface and uses
this data to run a Netflow-style cache before emitting flow data to the
pipeline. As opposed to the `bpf` segment, this one uses the classic packet
capture method and has no prospect of supporting hardware offloading. The segment
supports different methods:

- `pcapgo`, the only completely CGO-free, pure-Go method that should work anywhere, but does not support BPF filters
- `pcap`, a wrapper around libpcap, requires that at compile- and runtime
- `pfring`, a wrapper around PF_RING, requires the appropriate libraries as well as the loaded kernel module
- `file`, a `pcapgo` replay reader for PCAP files which will fallback to `pcap` automatically if either:
  1. the file is not in `.pcapng` format, but using the legacy `.pcap` format
  2. a BPF filter was specified

The filter parameter available for some methods will filter packets before they are
aggregated in any flow cache.

<details>
<summary>Configuration options</summary>

* **Method** _string_
* **Source** _string_
* **Filter** _string_
* **ActiveTimeout** _string_
* **InactiveTimeout** _string_

</details>

#### replay

_This segment is implemented in [replay.go](https://github.com/BelWue/flowpipeline/tree/master/segments/input/replay/replay.go)._

The `replay` segment reads a sqlite database previously created by the `sqlite`
segment and emits the flows contained in it. The location of the database is
specified with the `filename` parameter. Note that as of now, the database must
contain all columns/fields that are exported from the `EnrichedFlow` type. If
`respecttiming` is set to `true`, the segment will respect the timing of the original
flows and will replay them accordingly. Otherwise, the segment will emit all flows
instantly after each other.

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **RespectTiming** _bool_

</details>

#### stdin

_This segment is implemented in [stdin.go](https://github.com/BelWue/flowpipeline/tree/master/segments/input/stdin/stdin.go)._

The `stdin` segment reads JSON encoded flows from stdin or a given file and introduces
this into the pipeline. This is intended to be used in conjunction with the `json`
segment, which allows flowpipelines to be piped into each other. This segment can
also read files created with the `json` segment. The `eofcloses` parameter can
therefore be used to gracefully terminate the pipeline after reading the file.

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **EofCloses** _bool_

</details>

### Meta Group

Segments in this group are used for exporting meta data about the flowpipeline itself.

#### Monitoring Group

_No group documentation found._

### Modify Group

Segments in this group modify flows in some way. Generally, these segments do not drop
flows unless specifically instructed and only change fields within them. This group
contains both, enriching and reducing segments.

#### addcid

_This segment is implemented in [addcid.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/addcid/addcid.go)._

Deprecated: This segment was very specific and was replaced by a more generic
addnetid segment. Set useintids to true to have a similar behaviour to addcid.

The `addcid` segment can add a customer ID to flows according to the IP prefix
the flow is matched to. These prefixes are sourced from a simple csv file
consisting of lines in the format `ip prefix,integer`. For example:

```csv
192.168.88.0/24,1
2001:db8:1::/48,1
```

Which IP address is matched against this data base is determined by the
RemoteAddress field of the flow. If this is unset, the flow is forwarded
untouched. To set this field, see the `remoteaddress` segment. If matchboth is
set to true, this segment will not try to establish the remote address and
instead check both, source and destination address, in this order on a
first-match basis. The assumption here is that there are no flows of customers
sending traffic to one another.

If dropunmatched is set to true no untouched flows will pass this segment,
regardless of the reason for the flow being unmatched (absence of RemoteAddress
field, actually no matching entry in data base).

Roadmap:
* figure out how to deal with customers talking to one another

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **DropUnmatched** _bool_
* **MatchBoth** _bool_

</details>

#### addnetid

_This segment is implemented in [addnetid.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/addnetid/addnetid.go)._

_No segment documentation found._

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **DropUnmatched** _bool_
* **MatchBoth** _bool_
* **UseIntIds** _bool_

</details>

#### addrstrings

_This segment is implemented in [addrstrings.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/addrstrings/addrstrings.go)._

The `addrstrings` segment adds string representations of IP and MAC addresses which
are set. The new fields are

* `SourceIP` (from `SrcAddr`)
* `DestinationIP` (from `DstAddr`)
* `NextHopIP` (from `NextHop`)
* `SamplerIP` (from `SamplerAddress`)

* `SourceMAC` (from `SrcMac`)
* `DestinationMAC` (from `DstMac`)

This segment has no configuration options. It is intended to be used in conjunction
with the `dropfields` segment to remove the original fields.

#### anonymize

_This segment is implemented in [anonymize.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/anonymize/anonymize.go)._

The `anonymize` segment anonymizes IP addresses occuring in flows using the
Crypto-PAn algorithm. By default all possible IP address fields are targeted,
this can be configured using the fields parameter. The key needs to be at least
32 characters long.

Supported Fields for anonymization are `SrcAddr,DstAddr,SamplerAddress,NextHop`

<details>
<summary>Configuration options</summary>

* **EncryptionKey** _string_
* **AnonymizationMode** _Mode_

</details>

#### aslookup

_This segment is implemented in [aslookup.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/aslookup/aslookup.go)._

The `aslookup` segment can add AS numbers to flows using route collector dumps.
Dumps can be obtained from your RIR in the `.mrt` format and can be converted to
lookup databases using the `asnlookup-util` from the `asnlookup` package. These
databases contain a mapping from IP ranges to AS number in binary format.

By default the type is set to `db`. It is possible to directly parse `.mrt` files,
however this is not recommended since this will significantly slow down lookup times.

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **Type** _string_

</details>

#### bgp

_This segment is implemented in [bgp.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/bgp/bgp.go)._

The `bgp` segment can add a information from BGP to flows. By default, this
information is retrieved from a session with the router specified by a flow's
SamplerAddress.

To this end, this segment requires an additional configuration file for
configuring BGP sessions with routers. In below case, no SamplerAddress string
representation has been configured, but rather some other name ("default") to
be used in this segments fallback configuration parameter.

```yaml
routerid: "192.0.2.42"
asn: 553
routers:
  default:
    neighbors:
      - 192.0.2.1
      - 2001:db8::1
```

For the above bgp config to work, the parameter `fallbackrouter: default` is
required. This segment will first try to lookup a router by SamplerAddress, but
if no such router session is configured, it will fallback to the
`fallbackrouter` only if it is set. The parameter `usefallbackonly` is to
disable matching for SamplerAddress completely, which is a common use case and
makes things slightly more efficient.

If no `fallbackrouter` is set, no data will be annotated. The annotated fields are
`ASPath`, `Med`, `LocalPref`, `DstAS`, `NextHopAS`, `NextHop`, wheras the last
three are possibly overwritten from the original router export.

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **FallbackRouter** _string_
* **UseFallbackOnly** _bool_
* **RouterASN** _uint32_

</details>

#### dropfields

_This segment is implemented in [dropfields.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/dropfields/dropfields.go)._

The segment `dropfields` deletes fields from flows as they pass through this
segment. To this end, this segment requires a policy parameter to be set to
"keep" or "drop". It will then either keep or drop all fields specified in the
fields parameter. For a list of fields, check our
[protobuf definition](https://github.com/bwNetFlow/protobuf/blob/master/flow-messages-enriched.proto).

<details>
<summary>Configuration options</summary>

* **Policy** _Policy_

</details>

#### geolocation

_This segment is implemented in [geolocation.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/geolocation/geolocation.go)._

The `geolocation` segment annotates flows with their RemoteCountry field.
Requires the filename parameter to be set to the location of a MaxMind
geolocation file, as shown in our example.

For this to work, it requires RemoteAddress to be set in the flow if matchboth
is set to its default `false`. If matchboth is true, the result will be written for both
SrcAddr and DstAddr into SrcCountry and DstCountry. The dropunmatched parameter will
drop flows without any remote country data set.

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **DropUnmatched** _bool_
* **MatchBoth** _bool_

</details>

#### normalize

_This segment is implemented in [normalize.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/normalize/normalize.go)._

The `normalize` segment multiplies the Bytes and the Packets field by the flows
SamplingRate field. Additionally, it sets the Normalized field for this flow to 1.

The fallback parameter is for flows known to be sampled which do not include
the sampling rate for some reason.

Roadmap:
* replace Normalized with an OriginalSamplingRate field and set SamplingRate to 1
instead

<details>
<summary>Configuration options</summary>

* **Fallback** _uint64_

</details>

#### protomap

_This segment is implemented in [protomap.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/protomap/protomap.go)._

The `protomap` segment sets the ProtoName string field according to the Proto
integer field. Note that this should only be done before final usage, as
lugging additional string content around can be costly regarding performance
and storage size.

#### remoteaddress

_This segment is implemented in [remoteaddress.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/remoteaddress/remoteaddress.go)._

Determines the remote address of flows based on different criteria.

This segment basically runs flows through a switch case directive using its
only config parameter, 'policy'. Some policies unlock additional parameters.

  - 'cidr' matches first source address and then destination address against the
    prefixed provided in the CSV file using the config parameter 'filename'.
    When a match occurs, the RemoteAddress indicator is set
    accordingly. Accordingly means that the matching network is assumed to be
    remote. Optionally, unmatched flows can be dropped using 'dropunmatched'.
    If both match the source network is always picked as the remote network

  - 'border' assumes flows are exported on border interfaces: If a flow's
    direction is 'ingress' on such an interface, its remote address is the
    source address of the flow, whereas the local address of the flow would be
    its destination address inside our network. The same logic applies vice
    versa: 'egress' flows have a remote destination address.

  - 'user' assumes flows are exported on user interfaces: If a flow's
    direction is 'ingress' on such an interface, its remote address is the
    destination address of the flow, whereas the local address of the flow would be
    its destination address inside our user's network. The same logic applies
    vice versa: 'egress' flows have a remote source address.

  - 'clear' assumes flows are exported whereever, and thus all remote address
    info is cleared in this case.

Any optional parameters relate to the `cidr` policy only and behave as in the
`addnetid` segment.

<details>
<summary>Configuration options</summary>

* **Policy** _string_
* **FileName** _string_
* **DropUnmatched** _bool_

</details>

#### reversedns

_This segment is implemented in [reversedns.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/reversedns/reversedns.go)._

The `reversedns` segment looks up DNS PTR records for Src, Dst, Sampler and
NextHopAddr and adds them to our flows. The results are also written to a internal
cache which works well for ad-hoc usage, but it's recommended to use an actual
caching resolver in real deployment scenarios. The refresh interval setting pertains
to the internal cache only.

<details>
<summary>Configuration options</summary>

* **Cache** _bool_
* **RefreshInterval** _string_

</details>

#### snmp

_This segment is implemented in [snmp.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/snmp/snmp.go)._

The `snmp` segment annotates flows with interface information learned
directly from routers using SNMP. This is a potentially perfomance impacting
segment and has to be configured carefully.

In principle, this segment tries to fetch a SNMP OID datapoint from the address
in SamplerAddress, which corresponds to a router on normal flow-exporter
generated flows. The fields used to query this router are SrcIf and DstIf, i.e.
the interface IDs which are part of the flow. If successfull, a flow will have
the fields `{Src,Dst}IfName`, `{Src,Dst}IfDesc`, and `{Src,Dst}IfSpeed`
populated. In order to not to overload the router and to introduce delays, this
segment will:

  - not wait for a SNMP query to return, instead it will leave the flow as it was
    before sending it to the next segment (i.e. the first one on a given
    interface will always remain untouched)
  - add any interface's data to a cache, which will be used to enrich the
    next flow using that same interface
  - clear the cache value after 1 hour has elapsed, resulting in another flow
    without these annotations at that time

These rules are applied for source and destination interfaces separately.

The paramters to this segment specify the SNMPv2 community as well as the
connection limit employed by this segment. The latter is again to not overload
the routers SNMPd. Lastly, the regex parameter can be used to limit the
`IfDesc` annotations to a certain part of the actual interface description.
For instance, descriptions follow the format `customerid - blablalba`, the
regex `(.*) -.*` would grab just that customer ID to put into the `IfDesc`
fields. Also see the full examples linked below.

Roadmap:
* cache timeout should be configurable

<details>
<summary>Configuration options</summary>

* **Community** _string_
* **Regex** _string_
* **ConnLimit** _uint64_

</details>

#### sync_timestamps

_This segment is implemented in [sync_timestamps.go](https://github.com/BelWue/flowpipeline/tree/master/segments/modify/sync_timestamps/sync_timestamps.go)._

The segment `sync_timestamps` tries to fill empty time fields using existing ones.
It works on the following fields:
- TimeFlowStart:
  - TimeFlowStart
  - TimeFlowStartMs
  - TimeFlowStartNs
- TimeFlowEnd:
  - TimeFlowEnd
  - TimeFlowEndMs
  - TimeFlowEndNs
- TimeReceived:
  - TimeReceived
  - TimeReceivedNs

### Output Group

Segments in this group export flows to external tools, databases or file-storage. As
all other segments do, these still forward incoming flows to the next segment, i.e.
multiple output segments can be used in sequence to export to different places.

#### clickhouse

_This segment is implemented in [clickhouse.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/clickhouse/clickhouse.go)._

The `clickhouse` segment dumps all incoming flow messages to a clickhouse database.

The `batchsize` parameter determines the number of flows stored in memory before writing them to the database. Default is 1000.\
The `dsn` parameter is used to specify the `Data Source Name` of the clickhouse database to which the flows should be dumped.\
The `preset` parameter is used to specify the schema used to insert into clickhouse. Currently only the default value `flowhouse` is supported.

<details>
<summary>Configuration options</summary>

* **DSN** _string_
* **Preset** _string_
* **BatchSize** _int_

</details>

#### csv

_This segment is implemented in [csv.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/csv/csv.go)._

The `csv` segment provides an CSV output option. It uses stdout by default, but
can be instructed to write to file using the filename parameter. The fields
parameter can be used to limit which fields will be exported. If no filename is
provided or empty, the output goes to stdout. By default all fields are exported.
To reduce them, use a valid comma separated list of fields.

<details>
<summary>Configuration options</summary>

* **File** _*os.File_: Optional output file. If not set, stdout is used.
* **Fields** _string_

</details>

#### influx

_This segment is implemented in [influx.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/influx/influx.go)._

The `influx` segment provides a way to write into an Influxdb instance.
The `tags` parameter allows any field to be used as a tag and takes a comma-separated list from any
field available in the [protobuf definition](https://github.com/BelWue/flowpipeline/blob/master/pb/flow.proto).
The `fields` works in the exact same way, except that these protobuf fields won't be indexed by InfluxDB.

Note that some of the above fields might not be present depending on the method
of flow export, the input segment used in this pipeline, or the modify segments
in front of this export segment.

<details>
<summary>Configuration options</summary>

* **Address** _string_
* **Org** _string_
* **Bucket** _string_
* **Token** _string_

</details>

#### json

_This segment is implemented in [json.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/json/json.go)._

The `json` segment provides a JSON output option. It uses stdout by default, but can
be instructed to write to file using the filename parameter. This is intended to be
able to pipe flows between instances of flowpipeline, but it is also very useful when
debugging flowpipelines or to create a quick plaintext dump.

If the option `zstd` is set, the output will be compressed using the
[zstandard algorithm](https://facebook.github.io/zstd/). If the option `zstd` is set
to a positive integer, the compression level will be set to
([approximately](https://github.com/klauspost/compress/tree/master/zstd#status)) that
value. When `flowpipeline` is stopped abruptly (e.g by pressing Ctrl+C), the end of
the archive will get corrupted. Simply use `zstdcat` to decompress the archive and
remove the last line (`| head -n -1`).

If the option `pretty` is set to true, the every flow will be formatted in a
human-readable way (indented and with line breaks). When omitted, the output will be
a single line per flow.

<details>
<summary>Configuration options</summary>

* **File** _*os.File_: Optional output file. If not set, stdout is used.
* **Pretty** _bool_

</details>

#### kafkaproducer

_This segment is implemented in [kafkaproducer.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/kafkaproducer/kafkaproducer.go)._

The `kafkaproducer` segment produces flows to a Kafka topic. All settings are
equivalent to the `kafkaconsumer` segment. Additionally, there is the
`topicsuffix` parameter, which allows pipelines to write to multiple topics at
once. A typical use case is this:

  - set `topic: customer-`
  - set `topicsuffix: Cid`
  - this will result in a number of topics which will a) be created or b) need to
    exist, depending on your Kafka cluster settings.
  - the topics will take the form `customer-123` for all values of Cid
  - it is advisable to use the `flowfilter` segment to limit the number of
    topics

This could also be used to populate topics by Proto, or by Etype, or by any
number of other things.

<details>
<summary>Configuration options</summary>

* **Server** _string_
* **Topic** _string_
* **TopicSuffix** _string_
* **User** _string_
* **Pass** _string_
* **Tls** _bool_
* **Auth** _bool_
* **Legacy** _bool_
* **KafkaVersion** _string_

</details>

#### lumberjack

_This segment is implemented in [lumberjack.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/lumberjack/lumberjack.go)._

Send passing flows to one or more lumberjack (Elastic Beats) servers.

<details>
<summary>Configuration options</summary>

* **BatchSize** _int_

</details>

#### Mongodb Group

_No group documentation found._

#### prometheus

_This segment is implemented in [prometheus.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/prometheus/prometheus.go)._

The `prometheus` segment provides a standard prometheus exporter, exporting its
own monitoring info at `:8080/metrics` and its flow data at `:8080/flowdata` by
default. The label set included with each metric is freely configurable with a
comma-separated list from any field available in the [protobuf definition](https://github.com/BelWue/flowpipeline/blob/master/pb/flow.proto).

Note that some of the above fields might not be present depending on the method
of flow export, the input segment used in this pipeline, or the modify segments
in front of this export segment.

<details>
<summary>Configuration options</summary>

* **Endpoint** _string_
* **MetricsPath** _string_
* **FlowdataPath** _string_
* **ExportASPathPairs** _bool_
* **ExportASPaths** _bool_

</details>

#### sqlite

_This segment is implemented in [sqlite.go](https://github.com/BelWue/flowpipeline/tree/master/segments/output/sqlite/sqlite.go)._

**This segment is unavailable in the static binary release due to its CGO dependency.**

The `sqlite` segment provides a SQLite output option. It is intended for use as
an ad-hoc dump method to answer questions on live traffic, i.e. average packet
size for a specific class of traffic. The fields parameter optionally takes a
string of comma-separated fieldnames, e.g. `SrcAddr,Bytes,Packets`.

The batchsize parameter determines the number of flows stored in memory before
writing them to the database in a transaction made up from as many insert
statements. For the default value of 1000 in-memory flows, benchmarks show that
this should be an okay value for processing at least 1000 flows per second on
most szenarios, i.e. flushing to disk once per second. Mind the expected flow
throughput when setting this parameter.

<details>
<summary>Configuration options</summary>

* **FileName** _string_
* **Fields** _string_
* **BatchSize** _int_

</details>

### pass

_This segment is implemented in [pass.go](https://github.com/BelWue/flowpipeline/tree/master/segments/pass/pass.go)._

The `pass` segment serves as a heavily annotated template for new segments. So
does this piece of documentation. Aside from summarizing what a segment does,
it should include a description of all the parameters it accepts as well as any
caveats users should be aware of.

### Print Group

Segments in this group serve to print flows immediately to the user or a file. This is intended
for ad-hoc applications and instant feedback use cases.

#### count

_This segment is implemented in [count.go](https://github.com/BelWue/flowpipeline/tree/master/segments/print/count/count.go)._

The `count` segment counts flows passing it. This is mainly for debugging
flowpipelines. For instance, placing two of these segments around a
`flowfilter` segment allows users to use the `prefix` parameter with values
`"pre-filter: "`  and `"post-filter: "` to obtain a count of flows making it
through the filter without resorting to some command employing `| wc -l`.

The result is printed upon termination of the flowpipeline or to a file if a
filename is configured.

<details>
<summary>Configuration options</summary>

* **File** _*os.File_: Optional output file. If not set, stdout is used.
* **Prefix** _string_

</details>

#### printdots

_This segment is implemented in [printdots.go](https://github.com/BelWue/flowpipeline/tree/master/segments/print/printdots/printdots.go)._

The `printdots` segment keeps counting flows internally and emits a dot (`.`) to
stdout every `flowsperdot` flows. Its parameter needs to be chosen with the expected
flows per second in mind to be useful. Used to get visual feedback when
necessary. The segment can also print to a file if a filename is configured.

<details>
<summary>Configuration options</summary>

* **File** _*os.File_: Optional output file. If not set, stdout is used.
* **FlowsPerDot** _uint64_

</details>

#### printflowdump

_This segment is implemented in [printflowdump.go](https://github.com/BelWue/flowpipeline/tree/master/segments/print/printflowdump/printflowdump.go)._

The `printflowdump` prints a tcpdump-style representation of flows with some
addition deemed useful at [BelWü](https://www.belwue.de) Ops. It looks up
protocol names from flows if the segment `protoname` is part of the pipeline so
far or determines them on its own (using the same method as the `protoname`
segment), unless configured otherwise.

It currently looks like `timereceived: SrcAddr -> DstAddr [ingress iface desc from snmp segment →
@SamplerAddress → egress iface desc], ProtoName, Duration, avg bps, avg pps`. In
action:

```
14:47:41: x.x.x.x:443 -> 193.197.x.x:9854 [Telia → @193.196.190.193 → stu-nwz-a99], TCP, 52s, 52.015384 Mbps, 4.334 kpps
14:47:49: 193.197.x.x:54643 -> x.x.x.x:7221 [Uni-Ulm → @193.196.190.2 → Telia], UDP, 60s, 2.0288 Mbps, 190 pps
14:47:49: 2003:x:x:x:x:x:x:x:51052 -> 2001:7c0:x:x::x:443 [DTAG → @193.196.190.193 → stu-nwz-a99], UDP, 60s, 29.215333 Mbps, 2.463 kpps
```

This segment is commonly used as a pipeline with some input provider and the
`flowfilter` segment preceeding it, to create a tcpdump-style utility. This is
even more versatile when using `$0` as a placeholder in `flowfilter` to use the
flowpipeline invocation's first argument as a filter.

The parameter `verbose` changes some output elements, it will for instance add
the decoded forwarding status (Cisco-style) in a human-readable manner. The
`highlight` parameter causes the output of this segment to be printed in red,
see the [relevant example](https://github.com/BelWue/flowpipeline/tree/master/examples/configuration/highlighted_flowdump)
for an application. The parameter `filename`can be used to redirect the output to a file instead of printing it to stdout.

<details>
<summary>Configuration options</summary>

* **File** _*os.File_: Optional output file. If not set, stdout is used.
* **UseProtoname** _bool_
* **Verbose** _bool_
* **Highlight** _bool_

</details>

#### toptalkers

_This segment is implemented in [toptalkers.go](https://github.com/BelWue/flowpipeline/tree/master/segments/print/toptalkers/toptalkers.go)._

The `toptalkers` segment prints a report on which destination addresses
receives the most traffic. A report looks like this:

```
x.x.x.x: 734.515139 Mbps, 559.153067 kpps
x.x.x.x: 654.705813 Mbps, 438.586667 kpps
x.x.x.x: 507.164314 Mbps, 379.857067 kpps
x.x.x.x: 463.91171 Mbps, 318.9248 kpps
...
```

One can configure the sliding window size using `window`, as well as the
`reportinterval`. Optionally, this segment can report its output to a file and
use a custom prefix for any of its lines in order to enable multiple segments
writing to the same file. The thresholds serve to only log when the largest top
talkers are of note: the output is suppressed when either bytes or packets per
second are under their thresholds.

<details>
<summary>Configuration options</summary>

* **File** _*os.File_: Optional output file. If not set, stdout is used.
* **Window** _int_
* **ReportInterval** _int_
* **LogPrefix** _string_
* **ThresholdBps** _uint64_
* **ThresholdPps** _uint64_
* **TopN** _uint64_

</details>

### Testing Group

_No group documentation found._

#### generator

_This segment is implemented in [generator.go](https://github.com/BelWue/flowpipeline/tree/master/segments/testing/generator/generator.go)._

_No segment documentation found._