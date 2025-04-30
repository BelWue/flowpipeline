//go:build linux
// +build linux

// based on https://github.com/bwNetFlow/bpf_flowexport/blob/master/packetdump/packetdump.go
package bpf

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
)

// $BPF_CLANG and $BPF_CFLAGS are set by the Makefile.
//
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang-13 -cflags "-O2 -g -Wall -Werror" bpf ./bpf/bpf.c
type rawPacket struct {
	SrcAddrHi     uint64
	SrcAddrLo     uint64
	DstAddrHi     uint64
	DstAddrLo     uint64
	InIf          uint32
	OutIf         uint32
	Bytes         uint32
	Etype         uint32
	Proto         uint32
	Ipv6FlowLabel uint32
	SrcAddr       uint32
	DstAddr       uint32
	SrcPort       uint16
	DstPort       uint16
	IPTtl         uint8
	IPTos         uint8
	IcmpType      uint8
	IcmpCode      uint8
	TcpFlags      uint8
	FlowDirection uint8
	RemoteAddr    uint8
}

type Packet struct {
	SrcAddr       net.IP
	DstAddr       net.IP
	InIf          uint32
	OutIf         uint32
	Bytes         uint32
	Etype         uint32
	Proto         uint32
	Ipv6FlowLabel uint32
	SrcPort       uint16
	DstPort       uint16
	IPTtl         uint8
	IPTos         uint8
	IcmpType      uint8
	IcmpCode      uint8
	TcpFlags      uint8
	FlowDirection uint8
	RemoteAddr    uint8
}

func parseRawPacket(rawPacket rawPacket) Packet {
	var srcip, dstip net.IP
	if rawPacket.Etype == 0x0800 {
		srcip = make(net.IP, 4)
		binary.BigEndian.PutUint32(srcip, rawPacket.SrcAddr)
		dstip = make(net.IP, 4)
		binary.BigEndian.PutUint32(dstip, rawPacket.DstAddr)
	} else if rawPacket.Etype == 0x86dd {
		srcip = make(net.IP, 16)
		binary.BigEndian.PutUint64(srcip, rawPacket.SrcAddrHi)
		binary.BigEndian.PutUint64(srcip[8:], rawPacket.SrcAddrLo)
		dstip = make(net.IP, 16)
		binary.BigEndian.PutUint64(dstip, rawPacket.DstAddrHi)
		binary.BigEndian.PutUint64(dstip[8:], rawPacket.DstAddrLo)
	}
	return Packet{SrcAddr: srcip,
		DstAddr:       dstip,
		InIf:          rawPacket.InIf,
		OutIf:         rawPacket.OutIf,
		Bytes:         rawPacket.Bytes,
		Etype:         rawPacket.Etype,
		Proto:         rawPacket.Proto,
		Ipv6FlowLabel: rawPacket.Ipv6FlowLabel,
		SrcPort:       rawPacket.SrcPort,
		DstPort:       rawPacket.DstPort,
		IPTtl:         rawPacket.IPTtl,
		IPTos:         rawPacket.IPTos,
		IcmpType:      rawPacket.IcmpType,
		IcmpCode:      rawPacket.IcmpCode,
		TcpFlags:      rawPacket.TcpFlags,
		FlowDirection: rawPacket.FlowDirection,
		RemoteAddr:    rawPacket.RemoteAddr,
	}
}

type PacketDumper struct {
	BufSize int // Determines kernel perf map allocation. It is rounded up to the nearest multiple of the current page size.

	packets chan Packet // exported through .Packets()

	// setup
	objs           bpfObjects
	socketFilterFd int
	iface          *net.Interface
	SamplerAddress net.IP

	// start
	socketFd   int
	perfReader *perf.Reader
}

func (b *PacketDumper) Packets() chan Packet {
	if b.packets != nil {
		return b.packets
	}
	b.packets = make(chan Packet)
	go func() {
		var rawPacket rawPacket
		for {
			record, err := b.perfReader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					return
				}
				log.Error().Err(err).Msg("BPF packet dump: Error reading from kernel perf event reader: ")
				continue
			}

			if record.LostSamples != 0 {
				log.Warn().Msgf("BPF packet dump: Dropped %d samples from kernel perf buffer, consider increasing BufSize (currently %d bytes)", record.LostSamples, b.BufSize)
				continue
			}

			// Parse the perf event entry into an Event structure.
			if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &rawPacket); err != nil {
				log.Error().Err(err).Msg("BPF packet dump: Skipped 1 sample, error decoding raw perf event data: ")
				continue
			}
			b.packets <- parseRawPacket(rawPacket)
		}
	}()
	return b.packets
}

func (b *PacketDumper) Setup(device string) error {
	// allow the current process to lock memory for eBPF resources
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal().Err(err).Msg("BPF packet dump: Error during required memlock removal: ")
	}

	b.objs = bpfObjects{} // load pre-compiled programs and maps into the kernel
	if err := loadBpfObjects(&b.objs, nil); err != nil {
		log.Fatal().Err(err).Msg("BPF packet dump: Error loading objects: ")
	}

	var err error
	if b.iface, err = net.InterfaceByName(device); err != nil {
		return fmt.Errorf("[error] Unable to get interface, err: %v", err)
	}

	var addrs []net.Addr
	addrs, err = b.iface.Addrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			b.SamplerAddress = ipnet.IP
			break
		}
	}

	return nil
}

func (b *PacketDumper) Start() error {
	var err error
	// 768 is the network byte order representation of 0x3 (constant syscall.ETH_P_ALL)
	if b.socketFd, err = syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, 768); err != nil {
		return fmt.Errorf("[error] Unable to open raw socket, err: %v", err)
	}

	sll := syscall.SockaddrLinklayer{
		Ifindex:  b.iface.Index,
		Protocol: 768,
	}
	if err := syscall.Bind(b.socketFd, &sll); err != nil {
		return fmt.Errorf("[error] Unable to bind interface to raw socket, err: %v", err)
	}

	if err := syscall.SetsockoptInt(b.socketFd, syscall.SOL_SOCKET, 50, b.objs.PacketDump.FD()); err != nil {
		return fmt.Errorf("[error] Unable to attach BPF socket filter: %v", err)
	}

	if b.BufSize == 0 {
		// Set to a reasonably conservative value thats safe for
		// client-side pps levels.
		// Cilium docs set this to os.Getpagesize, which will return
		// 4096 bytes on any resonably modern platform. This however
		// leads to dropped samples in high load situations for dev
		// machines and containers, so the default is to increase that
		// by a factor of 2.
		b.BufSize = 2 * 4096
	}
	b.perfReader, err = perf.NewReader(b.objs.Packets, b.BufSize)
	if err != nil {
		return fmt.Errorf("[error] Unable to connect kernel perf event reader: %s", err)
	}

	return nil
}

func (b *PacketDumper) Stop() {
	b.objs.Close()
	syscall.Close(b.socketFd)
	b.perfReader.Close()
}

// bpfObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfObjects struct {
	bpfPrograms
	bpfMaps
}
type bpfPrograms struct {
	PacketDump *ebpf.Program `ebpf:"packet_dump"`
}

// Close implements io.Closer.
func (b *bpfPrograms) Close() error {
	panic("unimplemented")
}

type bpfMaps struct {
	Packets *ebpf.Map `ebpf:"packets"`
}

// Close implements io.Closer.
func (b *bpfMaps) Close() error {
	panic("unimplemented")
}

func loadBpfObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadBpf()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// loadBpf returns the embedded CollectionSpec for bpf.
func loadBpf() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_BpfBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("[error] can't load bpf: %w", err)
	}

	return spec, err
}

func (o *bpfObjects) Close() error {
	return _BpfClose(
		&o.bpfPrograms,
		&o.bpfMaps,
	)
}

func _BpfClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed bpf_bpfeb.o
var _BpfBytes []byte
