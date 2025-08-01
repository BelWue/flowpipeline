# flowpipeline Configuration and User Guide

Any flowpipeline is configured in a single yaml file which is either located in
the default `config.yml` or specified using the `-c` option when calling the
binary. The config file contains a single list of so-called segments, which
are processing flows in order. Flows represented by
[protobuf messages](https://github.com/bwNetFlow/protobuf/blob/master/flow-messages-enriched.proto)
within the pipeline.

Usually, the first segment is from the input group, followed by any number of
different segments. Often, flowpipelines end with a segment from the output,
print, or export groups. All segments, regardless from which group, accept and
forward their input from previous segment to their subsequent segment, i.e.
even input or output segments can be chained to one another or be placed in the
middle of a pipeline.

A list of full configuration examples with their own explanations can be found
[here](https://github.com/BelWue/flowpipeline/tree/master/examples).

## Variable Expansion

Users can freely use environment variables in the `config` sections of any
segment. Additionally it is possible to reference command line arguments that
flowpipeline was invoked with that have not been parsed by the binary itself.
For instance:

```yaml
- segment: kafkaconsumer
  config:
    user: myself
    pass: $PASSWORD
```

```yaml
- segment: flowfilter
  config:
    filter: $0
```

```yaml
- segment: bpf
  config:
    device: $0
```

## Parallel execution
```yaml
- segment: segment_name
  jobs: 5
```
The `jobs` parameter allows you to configure the number of parallel instances of a segment that should be run. The application will create the specified number of parallel instances of the segment and distribute the workload across them. This can significantly reduce the overall processing time, especially for large or complex segments.
If not set, the default value is 1.
Note that using too many parallel instances can also lead to performance degradation, as the overhead of managing the parallel processes may outweigh the benefits. 

## Available Segments

In addition to this section the detailed godoc can be used to get an overview
of available segments by going
[here](https://pkg.go.dev/github.com/BelWue/flowpipeline#section-directories)
and clicking `Expand all` in the bottom right.

### Alert Group
Segments in this group alert external endpoints via different channels. They're
usually placed behind some form of filtering and include data from flows making
it to this segment in their notification.

#### http
The `http` segment is currently a work in progress and is limited to sending
post requests with the full flow data to a single endpoint at the moment. The
roadmap includes various features such as different methods, builtin
conditional, limiting payload data, and multiple receivers.

```yaml
- segment: http
  config:
    url: https://example.com/postable-endpoint
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/alert/http)
[examples using this segment](https://github.com/search?q=%22segment%3A+http%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

### Analysis Group
Segments in this group do higher level analysis on flow data. They usually
export or print results in some way, but might also filter given flows.

#### toptalkers_metrics
The `toptalkers-metrics` segment calculates statistics about traffic levels
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

```yaml
- segment: toptalkers_metrics
  config:
    # the lines below are optional and set to default
    traffictype: ""
    buckets: 60
    BucketDuration: 1
    Thresholdbuckets: 60
    reportbuckets: 60
    thresholdbps: 0
    thresholdpps: 0
    endpoint: ":8080"
    metricspath: "/metrics"
    flowdatapath: "/flowdata"
    relevantaddress: "destination"
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/analysis/toptalkers-metrics)
[examples using this segment](https://github.com/search?q=%22segment%3A+toptalkers-metrics%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### traffic_specific_toptalkers
The traffic specific toptalker metrics segement is simmilar to the `toptalker-metrics` segment, 
but allows filtering for specific protocols. The use of nested filters is supported to
allow for a more efficient filtering.

Filters with a specified `traffictyp` will be exported if they reach the configured thresholds.

```yaml
- segment: traffic_specific_toptalkers
  config:
    endpoint: ":8085"
    traffic_specific_toptalkers:
    - filter: "proto udp"
      subfilter:
      - filter: "port 0"
        traffictype: "UDP Fragmented"
        thresholdbps: 100
      - filter: "port 53"
        traffictype: "DNS"
        filter: "port 53"
        thresholdbps: 100
      - filter: "port 123"
        traffictype: "NTP"
        thresholdbps: 100
      - filter: "port 389"
        traffictype: "CLDAP"
        thresholdbps: 100
    - filter: "proto icmp or proto icmpv6"
      traffictype: "ICMP"
      thresholdpps: 100
```

### Controlflow Group
Segments in this group have the ability to change the sequence of segments any
given flow traverses.

#### branch
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

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/controlflow/branch)
[examples using this segment](https://github.com/search?q=%22segment%3A+branch%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)


#### skip

*DEPRECATION NOTICE*: This segment will be deprecated in a future version of
flowpipeline. In any but the most convoluted examples, the `branch` segment
documented directly above is the clearer and more legible choice. This will
also greatly simplify the setup internals of segments.


The `skip` segment is used to conditionally skip over segments behind it. For
instance, in front of a export segment a condition such as `proto tcp` with a
skip value of `1` would result in any TCP flows not being exported by the
following segment. Setting invert to `true` is equivalent to negating the
condition.

```yaml
- segment: skip
  config:
    condition: `proto tcp`
    # the lines below are optional and set to default
    skip: 1
    invert: false
```

[flowfilter syntax](https://github.com/BelWue/flowfilter)
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/controlflow/skip)
[examples using this segment](https://github.com/search?q=%22segment%3A+skip%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

### Export Group
Segments in this group export flows to external databases. The distinction from
the output group lies in the fact that these exports are potentially lossy,
i.e. some fields might be lost. For instance, the `prometheus` segment as a
metric provider does not export any information about flow timing or duration,
among others.

#### influx
The `influx` segment provides a way to write into an Influxdb instance.
The `tags` parameter allows any field to be used as a tag and takes a comma-separated list from any
field available in the [protobuf definition](https://github.com/BelWue/flowpipeline/blob/master/pb/flow.proto).
The `fields` works in the exact same way, except that these protobuf fields won't be indexed by InfluxDB.

Note that some of the above fields might not be present depending on the method
of flow export, the input segment used in this pipeline, or the modify segments
in front of this export segment.

```yaml
- segment: influx
  config:
    org: my-org
    bucket: my-bucket
    token: $AUTH_TOKEN_ENVVAR
    # the lines below are optional and set to default
    address: http://127.0.0.1:8086
    tags: "ProtoName"
    fields: "Bytes,Packets"
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/export/prometheus)
[examples using this segment](https://github.com/search?q=%22segment%3A+prometheus%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)


#### prometheus
The `prometheus` segment provides a standard prometheus exporter, exporting its
own monitoring info at `:8080/metrics` and its flow data at `:8080/flowdata` by
default. The label set included with each metric is freely configurable with a
comma-separated list from any field available in the [protobuf definition](https://github.com/BelWue/flowpipeline/blob/master/pb/flow.proto).

Note that some of the above fields might not be present depending on the method
of flow export, the input segment used in this pipeline, or the modify segments
in front of this export segment.

```yaml
- segment: prometheus
  config:
    # the lines below are optional and set to default
    endpoint: ":8080"
    labels: "Etype,Proto"
    metricspath: "/metrics"
    flowdatapath: "/flowdata"
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/export/prometheus)
[examples using this segment](https://github.com/search?q=%22segment%3A+prometheus%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

### Filter Group
Segments in this group all drop flows, i.e. remove them from the pipeline from
this segment on. Fields in individual flows are never modified, only used as
criteria.

#### drop
The `drop` segment is used to drain a pipeline, effectively starting a new
pipeline after it. In conjunction with `skip`, this can act as a `flowfilter`.

```yaml
- segment: drop
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/filter/drop)
[examples using this segment](https://github.com/search?q=%22segment%3A+drop%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### elephant
The `elephant` segment uses a configurable sliding window to determine flow
statistics at runtime and filter out unremarkable flows from the pipeline. This
segment can be configured to look at different aspects of single flows, i.e.
either the plain byte/packet counts or the average of those per second with
regard to flow duration. By default, it drops the lower 99% of flows with
regard to the configured aspect and does not use exact percentile matching,
instead relying on the much faster P-square estimation. For quick ad-hoc usage,
it can be useful to adjust the window size (in seconds).
The ramp up time defults to 0 (disabled), but can be configured to wait for analyzing flows. All flows within this Timerange are dropped after the start of the pipeline.

```yaml
- segment: elephant
  # the lines below are optional and set to default
  config:
    aspect: "bytes"
    percentile: 99.00
    exact: false
    window: 300
    rampuptime: 0
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/elephant)
[examples using this segment](https://github.com/search?q=%22segment%3A+elephant%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### flowfilter
The `flowfilter` segment uses
[flowfilter syntax](https://github.com/BelWue/flowfilter) to drop flows
based on the evaluation value of the provided filter conditional against any
flow passing through this segment.

```yaml
- segment: flowfilter
  config:
    filter: "proto tcp"
```

[flowfilter syntax](https://github.com/BelWue/flowfilter)
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/filter)
[examples using this segment](https://github.com/search?q=%22segment%3A+flowfilter%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

### Input Group
Segments in this group import or collect flows and provide them to all
following segments. As all other segments do, these still forward incoming
flows to the next segment, i.e. multiple input segments can be used in sequence
to add to a single pipeline.

#### bpf
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

```yaml
- segment: bpf
  config:
    # required fields
    device: eth0
    # the lines below are optional and set to default
    activetimeout: 30m
    inactivetimeout: 15s
    buffersize: 65536
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/input/bpf)
[examples using this segment](https://github.com/search?q=%22segment%3A+bpf%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### goflow
The `goflow` segment provides a convenient interface for
[goflow2](https://github.com/netsampler/goflow2) right from flowpipeline
config. It behaves closely like a regular goflow2 instance but provides flows
directly in our extended format (which has been based on goflow's from the
beginning of this project).

This flow collector needs to receive input from any IPFIX/Netflow/sFlow
exporters, for instance your network devices.

```yaml
- segment: goflow
  # the lines below are optional and set to default
  config:
    listen: "sflow://:6343,netflow://:2055"
    workers: 1
```

[goflow2 fields](https://github.com/netsampler/goflow2/blob/main/docs/protocols.md)
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/input/goflow)
[examples using this segment](https://github.com/search?q=%22segment%3A+goflow%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### kafkaconsumer
The `kafkaconsumer` segment consumes flows from a Kafka topic. This topic can
be created using the `kafkaproducer` module or using an external instance of
[goflow2](https://github.com/netsampler/goflow2).

This segment can be used in conjunction with the `kafkaproducer` segment to
enrich, reduce, or filter flows in transit between Kafka topics, or even sort
them into different Kafka topics. See the examples this particular usage.

The startat configuration sets whether to start at the newest or oldest
available flow (i.e. Kafka offset). It only takes effect if Kafka has no stored
state for this specific user/topic/consumergroup combination.

The supported group partion assignor balancing strategies can be set using a comma separated 
list for `strategy`. Supported values are `sticky`, `roundrobin` and `range`
Default is `sticky`.

```yaml
- segment: kafkaconsumer
  config:
    # required fields
    server: some.kafka.server.example.com:9092
    topic: flow-topic-name
    group: consumer-group-name
    # required if auth is true
    user: myusername
    pass: mypassword
    # the lines below are optional and set to default
    tls: true
    auth: true
    startat: newest
    kafka-version: 3.8.0
    timeout: 15s
    strategy: roundrobin,sticky
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/input/kafkaconsumer)
[examples using this segment](https://github.com/search?q=%22segment%3A+kafkaconsumer%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

##### BelWü-connected Entities
If you are connected behind [BelWü](https://www.belwue.de) contact one of our
developers or the [bwNet project](bwnet.belwue.de) for access to our central
Kafka cluster. We'll gladly create a topic for your university if it doesn't
exist already and provide you with the proper configuration to use this
segment.

If the person responsible at university agrees, we will pre-fill this topic
with flows sent or received by your university as seen on our upstream border
interfaces. This can be limited according to the data protection requirements
set forth by the universities.

#### packet
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

The filter parameter available for some methods will filter packets before they are aggregated in any flow cache.

```yaml
- segment: packet
  config:
    method: pcap # required, one of the available capture methods "pcapgo|pcap|pfring|file"
    source:      # required, for example "eth0" or "./dump.pcapng"
    # the lines below are optional and set to default
    filter: "" # optional pflang filter (libpcap's high-level BPF syntax), provided the method is libpcap, pfring, or file.
    activetimeout: 30m
    inactivetimeout: 15s
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/packet/bpf)
[examples using this segment](https://github.com/search?q=%22segment%3A+packet%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### replay
The `replay` segment reads a sqlite database previously created by the `sqlite` segment and emits the flows contained in it.
The location of the database is specified with the `filename` parameter.
Note that as of now, the database must contain all columns/fields that are exported from the `EnrichedFlow` type.
If `respecttiming` is set to `true`, the segment will respect the timing of the original flows and will replay them accordingly.
Otherwise, the segment will emit all flows instantly after each other.

```yaml
- segment: replay
  config:
    filename: dump.sqlite
    respecttiming: true # optional
```

#### stdin
The `stdin` segment reads JSON encoded flows from stdin or a given file and introduces this
into the pipeline. This is intended to be used in conjunction with the `json`
segment, which allows flowpipelines to be piped into each other.
This segment can also read files created with the `json` segment.
The `eofcloses` parameter can therefore be used to gracefully terminate the pipeline after reading the file.

```yaml
- segment: stdin
  # the lines below are optional and set to default
  config:
    filename: ""
    eofcloses: false
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/input/stdin)
[examples using this segment](https://github.com/search?q=%22segment%3A+stdin%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### diskbuffer

The `diskbuffer` segment buffers flows in memory and on-demand on disk.
Writing to disk is done in the JSON representation of the flows, compressed using `zstd`.
The flows are written to disk, when the MemoryBuffer reaches the percentual fill level HighMemoryMark,
until the LowMemoryMark is reached again.
Files are read from disk if the fill level reaches ReadingMemoryMark.
The maximum file size and the maximum size on disk are configurable via the `filesize` and `maxcachesize`
parameter.
If QueueStatusInterval is greater 0s, the fill level is printed.
BatchSize specifies how many flows will be at least written to disk

```yaml
- segment: diskbuffer
  config:
	bufferdir:           "" # must be specified, rest is optional
	batchsize:           128
	queuestatusinterval: 0s
	filesize:            50 MB
	highmemorymark:      70
	lowmemorymark:       30
	readingmemorymark:   5
	maxcachesize:        1 GB
	queuesize:           65536
```

### Modify Group
Segments in this group modify flows in some way. Generally, these segments do
not drop flows unless specifically instructed and only change fields within
them. This group contains both, enriching and reducing segments.

#### addcid
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

```yaml
- segment: addcid
  config:
    filename: filename.csv
    # the lines below are optional and set to default
    dropunmatched: false
    matchboth: false
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/addcid)
[examples using this segment](https://github.com/search?q=%22segment%3A+addcid%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### addrstrings

The `addrstrings` segment adds string representations of IP and MAC addresses which are set. The new fields are

* `SourceIP` (from `SrcAddr`)
* `DestinationIP` (from `DstAddr`)
* `NextHopIP` (from `NextHop`)
* `SamplerIP` (from `SamplerAddress`)

* `SourceMAC` (from `SrcMac`)
* `DestinationMAC` (from `DstMac`)

This segment has no configuration options. It is intended to be used in conjunction with the `dropfields` segment
to remove the original fields.

```yaml
- segment: addrstrings
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/addrstrings)

#### aslookup
The `aslookup` segment can add AS numbers to flows using route collector dumps.
Dumps can be obtained from your RIR in the `.mrt` format and can be converted to
lookup databases using the `asnlookup-util` from the `asnlookup` package. These
databases contain a mapping from IP ranges to AS number in binary format.

By default the type is set to `db`. It is possible to directly parse `.mrt` files,
however this is not recommended since this will significantly slow down lookup times.

```yaml
- segment: aslookup
  config:
    filename: ./lookup.db
    # the lines below are optional and set to default
    type: db # can be either db or mrt
```
[MRT specification](https://datatracker.ietf.org/doc/html/rfc6396)
[asnlookup](https://github.com/banviktor/asnlookup)
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/aslookup)
[examples using this segment](https://github.com/search?q=%22segment%3A+aslookup%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### bgp
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

```yaml
- segment: bgp
  config:
    filename: "bgp.conf"
    # the lines below are optional and set to default
    fallbackrouter: ""
    usefallbackonly: 0
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/bgp)
[examples using this segment](https://github.com/search?q=%22segment%3A+bgp%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### anonymize
The `anonymize` segment anonymizes IP addresses occuring in flows using the
Crypto-PAn algorithm. By default all possible IP address fields are targeted,
this can be configured using the fields parameter. The key needs to be at least
32 characters long.

Supported Fields for anonymization are `SrcAddr,DstAddr,SamplerAddress,NextHop`

```yaml
- segment: anonymize
  config:
    key: "abcdef"
    # the lines below are optional and set to default
    fields: "SrcAddr,DstAddr,SamplerAddress"
```

[CryptoPan module](https://github.com/Yawning/cryptopan)
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/anonymize)
[examples using this segment](https://github.com/search?q=%22segment%3A+anonymize%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### dropfields
The segment `dropfields` deletes fields from flows as they pass through this
segment. To this end, this segment requires a policy parameter to be set to
"keep" or "drop". It will then either keep or drop all fields specified in the
fields parameter. For a list of fields, check our
[protobuf definition](https://github.com/bwNetFlow/protobuf/blob/master/flow-messages-enriched.proto).

```yaml
- segment: dropfields
  config:
    # required, options are drop or keep
    policy: drop
    # required, fields to keep or drop
    fields: "SrcAddr,DstAddr"
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/dropfields)
[examples using this segment](https://github.com/search?q=%22segment%3A+dropfields%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### geolocation
The `geolocation` segment annotates flows with their RemoteCountry field.
Requires the filename parameter to be set to the location of a MaxMind
geolocation file, as shown in our example.

For this to work, it requires RemoteAddress to be set in the flow if matchboth
is set to its default `false`. If matchboth is true, the behaviour is
different from the `addcid` segment, as the result will be written for both
SrcAddr and DstAddr into SrcCountry and DstCountry. The dropunmatched parameter
however behaves in the same way: flows without any remote country data set will
be dropped.

```yaml
- segment: geolocation
  config:
    # required
    filename: file.mmdb
    # the lines below are optional and set to default
    dropunmatched: false
    matchboth: false
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/geolocation)
[examples using this segment](https://github.com/search?q=%22segment%3A+geolocation%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### normalize
The `normalize` segment multiplies the Bytes and the Packets field by the flows
SamplingRate field. Additionally, it sets the Normalized field for this flow to 1.

The fallback parameter is for flows known to be sampled which do not include
the sampling rate for some reason.

Roadmap:
* replace Normalized with an OriginalSamplingRate field and set SamplingRate to 1 instead

```yaml
- segment: normalize
  # the lines below are optional and set to default
  config:
    fallback: 0
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/normalize)
[examples using this segment](https://github.com/search?q=%22segment%3A+normalize%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### protomap
The `protomap` segment sets the ProtoName string field according to the Proto
integer field. Note that this should only be done before final usage, as
lugging additional string content around can be costly regarding performance
and storage size.

```yaml
- segment: protomap
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/protomap)
[examples using this segment](https://github.com/search?q=%22segment%3A+protomap%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### remoteaddress
The `remoteaddress` segment determines any given flows remote address and sets
the RemoteAddress field up to indicate either SrcAddr or DstAddr as the remote
address. This is done by different policies, see
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/remoteaddress)
for a detailed explanation.

The short version is:
* `cidr` assumes the remote address is the one that has a match in the CSV file
  used by the `addcid` segment. Done for source and destination addres.
* `border` assumes all flows originate at the outside network border, i.e. on
  peering, exchange, or transit interfaces. Thus, any incoming flows originate
  at a remote address (source address), and any outgoing flows originate at a
  local address (destination address is remote).
* `user` assumes the opposite, namely that flows are collected on user- or
  customer-facing interfaces. The assignment is thus reversed.
* `clear` clears all remote address info and is thus equivalent to using the
  `dropfields` segment. This can be used when processing flows from mixed
  sources before reestablishing remote address using `cidr`.

Any optional parameters relate to the `cidr` policy only and behave as in the
`addcid` segment.

```yaml
- segment: remoteaddress
  config:
    # required, one of cidr, border, user, or clear
    policy: cidr
    # required if policy is cidr
    filename: same_csv_file_as_for_addcid_segment.csv
    # the lines below are optional and set to default, relevant to policy cidr only
    dropunmatched: false
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/remoteaddress)
[examples using this segment](https://github.com/search?q=%22segment%3A+remoteaddress%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### reversedns
The `reversedns` segment looks up DNS PTR records for Src, Dst, Sampler and NextHopAddr and adds
them to our flows. The results are also written to a internal cache which works well for ad-hoc
usage, but it's recommended to use an actual caching resolver in real deployment scenarios. The
refresh interval setting pertains to the internal cache only.

```yaml
- segment: reversedns
  config:
    # the lines below are optional and set to default
    cache: true
    refreshinterval: 5m
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/reversedns)
[examples using this segment](https://github.com/search?q=%22segment%3A+reversedns%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### snmpinterface
The `snmpinterface` segment annotates flows with interface information learned
directly from routers using SNMP. This is a potentially perfomance impacting
segment and has to be configured carefully.

In principle, this segment tries to fetch a SNMP OID datapoint from the address
in SamplerAddress, which corresponds to a router on normal flow-exporter
generated flows. The fields used to query this router are SrcIf and DstIf, i.e.
the interface IDs which are part of the flow. If successfull, a flow will have
the fields `{Src,Dst}IfName`, `{Src,Dst}IfDesc`, and `{Src,Dst}IfSpeed`
populated. In order to not to overload the router and to introduce delays, this
segment will:

* not wait for a SNMP query to return, instead it will leave the flow as it was
  before sending it to the next segment (i.e. the first one on a given
  interface will always remain untouched)
* add any interface's data to a cache, which will be used to enrich the
  next flow using that same interface
* clear the cache value after 1 hour has elapsed, resulting in another flow
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

```yaml
- segment: snmpinterface
  # the lines below are optional and set to default
  config:
    community: public
    regex: ".*"
    connlimit: 16

```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/modify/snmp)
[examples using this segment](https://github.com/search?q=%22segment%3A+snmp%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### Sync Timestamps
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
  
### Meta Group
Segments in this group are used for exporting meta data about the flowpipeline itself

#### Delay Monitoring
The `delay_monitoring` segment measures how old the processed flows are and publishs the delay using a prometheus server.
The the delay is calculated using a exponential window moving average. The alpha value can be set using `alpha`.
To reduce load, a sampling intervall can be set using `samplingRate`.

```yaml
- segment: delay_monitoring
  # the lines below are optional and set to default
  config:
    endpoint: ":8080"
    samplingRate: 1000
    alpha: 0.2
```

### Output Group
Segments in this group export flows, usually while keeping all information
unless instructed otherwise. As all other segments do, these still forward
incoming flows to the next segment, i.e. multiple output segments can be used in
sequence to export to different places.

#### csv
The `csv` segment provides an CSV output option. It uses stdout by default, but
can be instructed to write to file using the filename parameter. The fields
parameter can be used to limit which fields will be exported.
If no filename is provided or empty, the output goes to stdout.
By default all fields are exported. To reduce them, use a valid comma seperated list of fields.

```yaml
- segment: csv
  # the lines below are optional and set to default
  config:
    filename: ""
    fields: ""
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/output/csv)
[examples using this segment](https://github.com/search?q=%22segment%3A+csv%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)


#### kafkaproducer
The `kafkaproducer` segment produces flows to a Kafka topic. All settings are
equivalent to the `kafkaconsumer` segment. Additionally, there is the
`topicsuffix` parameter, which allows pipelines to write to multiple topics at
once. A typical use case is this:
* set `topic: customer-`
* set `topicsuffix: Cid`
* this will result in a number of topics which will a) be created or b) need to
  exist, depending on your Kafka cluster settings.
* the topics will take the form `customer-123` for all values of Cid
* it is advisable to use the `flowfilter` segment to limit the number of
  topics

This could also be used to populate topics by Proto, or by Etype, or by any
number of other things.

```yaml
- segment: kafkaproducer
  config:
    # required fields
    server: some.kafka.server.example.com:9092
    topic: flow-topic-name
    # required if auth is true
    user: myusername
    pass: mypassword
    # the lines below are optional and set to default
    tls: true
    auth: true
    kafka-version: 3.8.0
    topicsuffix: ""
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/output/kafkaproducer)
[examples using this segment](https://github.com/search?q=%22segment%3A+kafkaproducer%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### sqlite
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

```yaml
- segment: sqlite
  config:
    filename: dump.sqlite
    # the lines below are optional and set to default
    fields: ""
    batchsize: 1000
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/output/sqlite)
[examples using this segment](https://github.com/search?q=%22segment%3A+sqlite%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### mongodb
**This segment is unavailable in the static binary release due to its CGO dependency.**

The `mongodb` segment provides a Mongodb output option. 
The connection parameter of the mongodb must be configured in the parameter `mongodb_uri`.
MongoDb is intended to be used as a ringbuffer like storage using capped collections.
The maximumg (unecrypted) disk usage can be set using `max_disk_usage`.
The fields parameter optionally takes a
string of comma-separated fieldnames, e.g. `SrcAddr,Bytes,Packets`.

The batchsize parameter determines the number of flows stored in memory before
writing them to the database in a transaction made up from as many insert
statements. For the default value of 1000 in-memory flows, benchmarks show that
this should be an okay value for processing at least 1000 flows per second on
most szenarios, i.e. flushing to disk once per second. Mind the expected flow
throughput when setting this parameter.

```yaml
- segment: mongodb
  config:
    mongodb_uri: "mongodb://localhost:27017/"
    max_disk_usage: "20 GB"
    fields: "SrcAddr,DstAddr,SrcPort,DstPort"

```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/output/sqlite)

#### json
The `json` segment provides a JSON output option.
It uses stdout by default, but can be instructed to write to file using the filename parameter.
This is intended to be able to pipe flows between instances of flowpipeline, but it is
also very useful when debugging flowpipelines or to create a quick plaintext
dump.

if the option `zstd` is set, the output will be compressed using the [zstandard algorithm](https://facebook.github.io/zstd/).
If the option `zstd` is set to a positive integer, the compression level will be set to
([approximately](https://github.com/klauspost/compress/tree/master/zstd#status)) that value.
When `flowpipeline` is stopped abruptly (e.g by pressing Ctrl+C), the end of the archive will get corrupted.
Simply use `zstdcat` to decompress the archive and remove the last line (`| head -n -1`).

If the option `pretty` is set to true, the every flow will be formatted in a human-readable way (indented and with line breaks).
When omitted, the output will be a single line per flow.

```yaml
- segment: json
  # the lines below are optional and set to default
  config:
    filename: ""
    zstd: 0
    pretty: false
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/output/json)
[examples using this segment](https://github.com/search?q=%22segment%3A+json%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### lumberjack (elastic beats)
The `lumberjack` segment sends flows to one or more [elastic beats](https://github.com/elastic/beats)
servers user the [lumberjack](https://github.com/logstash-plugins/logstash-input-beats/blob/main/PROTOCOL.md)
protocol. Flows are queued in a non-deterministic, round-robin fashion to the servers.

The only mandatory option is `servers` which contains a comma separated list of lumberjack
server URLs. Each URL must start with one of these schemata: `tcp://` (plain TCP,
no encryption), `tls://` (TLS encryption) or `tlsnoverify://` (TLS encryption without
certificate verification). The schema is followed by the hostname or IP address, a colon `:`,
and a port number. IPv6 addresses must be surrounded by square brackets.

A goroutine is spawned for every lumberjack server. Each goroutine only uses one CPU core to
process and send flows. This may not be enough when the ingress flow rate is high and/or a high compression
level is used. The number of goroutines per backend can by set explicitly with the `?count=x` URL
parameter. For example:

```yaml
config:
  server: tls://host1:5043/?count=4, tls://host2:5043/?compression=9&count=16
```

will use four parallel goroutines for `host1` and sixteen parallel goroutines for `host2`. Use `&count=…` instead of
`?count=…` when `count` is not the first parameter (standard URI convention).

Transport compression is disabled by default. Use `compression` to set the compression level
for all hosts. Compression levels can vary between 0 (no compression) and 9 (maximum compression).
To set per-host transport compression adding `?compression=<level>` to the server URI.

To prevent blocking, flows are buffered in a channel between the segment and the output
go routines. Each output go routine maintains a buffer of flows which are send either when the
buffer is full or after a configurable timeout. Proper parameter sizing for the queue,
buffers, and timeouts depends on multiple individual factors (like size, characteristics
of the incoming netflows and the responsiveness of the target servers). There are parameters
to both observe and tune this segment's performance.

Upon connection error or loss, the segment will try to reconnect indefinitely with a pause of
`reconnectwait` between attempts.

* `queuesize` (integer) sets the number of flows that are buffered between the segment and the output go routines.
* `batchsize` (integer) sets the number of flows that each output go routine buffers before sending.
* `batchtimeout` (duration) sets the maximum time that flows are buffered before sending.
* `reconnectwait` (duration) sets the time to wait between reconnection attempts.

These options help to observe the performance characteristics of the segment:

* `batchdebug` (bool) enables debug logging of batch operations (full send, partial send and skipped send).
* `queuestatusinterval` (duration) sets the interval at which the segment logs the current queue status.

To see debug output, set the `-l debug` flag when starting `flowpipeline`.

See [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) for proper duration format
strings and [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool) for allowed bool keywords.

```yaml
- segment: lumberjack
  config:
    servers: tcp://foo.example.com:5044, tls://bar.example.com:5044?compression=3, tlsnoverify://[2001:db8::1]:5044
    compression: 0
    batchsize: 1024
    queuesize: = 2048
    batchtimeout: "2000ms"
    reconnectwait: "1s"
    batchdebug: false
    queuestatusinterval: "0s"
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/output/lumberjack)

### Print Group
Segments in this group serve to print flows immediately to the user. This is intended for ad-hoc applications and instant feedback use cases.

#### count
The `count` segment counts flows passing it. This is mainly for debugging
flowpipelines. For instance, placing two of these segments around a
`flowfilter` segment allows users to use the `prefix` parameter with values
`"pre-filter: "`  and `"post-filter: "` to obtain a count of flows making it
through the filter without resorting to some command employing `| wc -l`.

The result is printed upon termination of the flowpipeline.

```yaml
- segment: count
  # the lines below are optional and set to default
  config:
    prefix: ""
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/print/count)
[examples using this segment](https://github.com/search?q=%22segment%3A+count%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### printdots
The `printdots` segment keeps counting flows internally and emits a dot (`.`)
every `flowsperdot` flows. Its parameter needs to be chosen with the expected
flows per second in mind to be useful. Used to get visual feedback when
necessary.

```yaml
- segment: printdots
  # the lines below are optional and set to default
  config:
    flowsperdot: 5000
```

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/print/printdots)
[examples using this segment](https://github.com/search?q=%22segment%3A+printdots%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### printflowdump
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
see the [relevant example](https://github.com/BelWue/flowpipeline/tree/master/examples/highlighted_flowdump)
for an application.

```yaml
- segment: printflowdump
  # the lines below are optional and set to default
  config:
    useprotoname: true
    verbose: false
    highlight: false
```
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/print/printflowdump)
[examples using this segment](https://github.com/search?q=%22segment%3A+printflowdump%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

#### toptalkers
The `toptalkers` segment prints a report on which destination addresses
receives the most traffic. A report looks like this:

```
===================================================================
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

```yaml
- segment: toptalkers
  # the lines below are optional and set to default
  config:
    window: 60
    reportinterval: 10
    filename: ""
    logprefix: ""
    thresholdbps: 0
    thresholdpps: 0
    topn: 10
```
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/print/toptalkers)
[examples using this segment](https://github.com/search?q=%22segment%3A+toptalkers%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)

### Ungrouped

This is for internally used segments only.

#### pass
The `pass` segment serves as a heavily annotated template for new segments. So
does this piece of documentation. Aside from summarizing what a segment does,
it should include a description of all the parameters it accepts as well as any
caveats users should be aware of.

Roadmap:
* things to expect go here
* and here

```yaml
- segment: pass
  # the lines below are optional and set to default
  config:
    jk: this segment actually has no config at all, its just for this template
```

[any additional links](https://bwnet.belwue.de)
[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/pass)
[examples using this segment](https://github.com/search?q=%22segment%3A+pass%22+extension%3Ayml+repo%3AbwNetFlow%2Fflowpipeline%2Fexamples&type=Code)
