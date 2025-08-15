# Flowpipeline
A [goflow2](https://github.com/netsampler/goflow2)-compatible flow message processing toolkit

[godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline)

----------

![Flowpipeline](https://github.com/user-attachments/assets/63c5ac84-b50d-442e-9641-c32fe95b58f4)

----------

## About The Project

[bwNET](https://bwnet.belwue.de/) is a research project of the German federal
state of Baden-Württemberg which aims to provide innovative services within the
state's research and education network [BelWü](https://www.belwue.de). This
GitHub Org contains the code pertaining to the monitoring aspect of this
project.

This repo contains our flow processing toolkit which enables us and our users
to define pipelines for [goflow2](https://github.com/netsampler/goflow2)-compatible
flow messages. The flowpipeline project integrates most other parts of our flow
processing stack into a single piece of software which can be configured to
serve any function:

* accepting raw Netflow (using [goflow2](https://github.com/netsampler/goflow2))
* enriching the resulting flow messages ([examples/enricher](https://github.com/BelWue/flowpipeline/tree/master/examples/enricher))
* writing to and reading from Kafka ([examples/localkafka](https://github.com/BelWue/flowpipeline/tree/master/examples/localkafka))
* dumping flows to cli (e.g. [flowdump](https://github.com/BelWue/flowpipeline/tree/master/examples/flowdump))
* providing metrics and insights ([examples/prometheus](https://github.com/BelWue/flowpipeline/tree/master/examples/prometheus))
* and many more...

## Getting Started

To get going, choose one of the following deployment methods.

### Compile from Source
Clone this repo and use `go build .` to build the binary yourself.

By default, the binary will look for a `config.yml` in its local directory, so
you'll either want to create one or call it from any example directory (and
maybe follow the instructions there).

### Binary Releases
Download our [latest release](https://github.com/BelWue/flowpipeline/releases)
and run it, same as if you compiled it yourself.
The flowpipeline releases contain executables for MacOS (`flowpipeline-darwin`) and for linux (`flowpipeline-linux`).

The default, dynamically linked version requires a reasonably recent system
(glibc 2.32+, linux 5.11+ for `bpf`, `mongodb` ...) and comes with all features.
As a fallback option, the static binaries will work in older environments
(Rocky Linux 8, Debian 11, ...), but come without the segments that require
CGO/dynamically linked code (`bpf`, `sqlite`, `mongodb` and plugin support, check
[CONFIGURATION.md](https://github.com/BelWue/flowpipeline/blob/master/CONFIGURATION.md)).

### Container Releases
A ready to use container is provided as `bwnetflow/flowpipeline`, you can check
it out on [GitHub container registry](https://github.com/BelWue/flowpipeline/pkgs/container/flowpipeline).

Configurations referencing other files (geolocation databases for instance)
will work in a container without extra edits. This is because the volume
mountpoint `/config` is prepended in all segments which accept configuration to
open files, if the binary was built with the `container` build flag.

```sh
podman run -v ./examples/xy:/config flowpipeline
# or
docker run -v ./examples/xy:/config flowpipeline
```

## Configuration

Refer to [CONFIGURATION.md](https://github.com/BelWue/flowpipeline/blob/master/CONFIGURATION.md)
for the full guide. Other than that, looking at the examples should give you a
good idea what the config looks like in detail and what the possible
applications are. 
The default configuration file used by flowpipeline is `config.yml`. 
To use another configuration file, its location has to be specified using `-c path/to/file.yml`.

A configuration file should start with a segment from the `input` group. This can then be followed
by one or more segments. The following segments process all flows output by the previous segment.
Most segments output all flows that they consumed from the previous segment. 
The exception to this are the `filter` group segments.

For sake of completeness, here's another minimal example
which starts listening for Netflow v9 on port 2055, applies the filter given as
first argument, and then prints it out to `stdout` in a `tcpdump`-style format.

```yaml
- segment: goflow

- segment: flowfilter
  config:
    filter: $0

- segment: printflowdump
```

You'd call it with `./flowpipeline "proto tcp and (port 80 or port 443)"`., for
instance.

### Production Deployment
For deployments in a production environment, the use of a central Kafka cluster is strongly advised.
This allows distributing multiple redundant flowpipeline instances throughout multiple georedundant locations.
The different workers can use the `kafkaconsumer` segment to read from and the `kafkaproducer` segment to write to the cluster.
### Custom Segments
If you find that the existing segments lack some functionality or you require
some very specific behaviour, it is possible to include segments as a plugin.
This is done using the `-p yourplugin.so` commandline option and your own
custom module. See
[examples/plugin](https://github.com/BelWue/flowpipeline/tree/master/examples/plugin)
for a basic example and instructions on how to compile your plugin.

Note that this requires CGO and thus will not work using the static binary
releases or in a container.

## Contributing

Contributions in any form (code, issues, feature requests) are very much welcome.
