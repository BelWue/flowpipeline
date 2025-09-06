package snmp

import (
	"testing"
)

// SNMP Segment test
// TODO: find a way to run this elsewhere, as this currently only works by
// having the local 161/udp port forwarded to some router.
// func TestSegment_SNMP(t *testing.T) {
// 	result := testSegmentWithFlows(
// 		&SNMP{
// 			Community: "public",
// 			Regex:     "^_[a-z]{3}_[0-9]{5}_[0-9]{5}_ [A-Z0-9]+ (.*?) *( \\(.*)?$",
// 			ConnLimit: 1,
// 		},
// 		[]*pb.EnrichedFlow{
// 			&pb.EnrichedFlow{Type: 42, SamplerAddress: []byte{127, 0, 0, 1}, InIf: 70},
// 			&pb.EnrichedFlow{SamplerAddress: []byte{127, 0, 0, 1}, InIf: 70},
// 		})
// 	if result.SrcIfDesc == "" {
// 		t.Error("([error] Segment SNMP is not adding a SrcIfDesc.")
// 	}
// }

func TestSegment_SNMP_instanciation(t *testing.T) {
	snmpInterface := &SNMP{}
	result := snmpInterface.New(map[string]string{})
	if result == nil {
		t.Error("([error] Segment SNMP did not initiate despite good base config.")
	}

	snmpInterface = &SNMP{}
	result = snmpInterface.New(map[string]string{"connlimit": "42"})
	if result == nil {
		t.Error("([error] Segment SNMP did not initiate despite good base config.")
	}

	snmpInterface = &SNMP{}
	result = snmpInterface.New(map[string]string{"community": "foo", "regex": ".*"})
	if result == nil {
		t.Error("([error] Segment SNMP did not initiate despite good config.")
	}

	snmpInterface = &SNMP{}
	result = snmpInterface.New(map[string]string{"community": "foo", "regex": "("})
	if result != nil {
		t.Error("([error] Segment SNMP did initiate despite bad regex config.")
	}

	snmpInterface = &SNMP{}
	result = snmpInterface.New(map[string]string{"connlimit": "-8"})
	if result == nil {
		t.Error("([error] Segment SNMP did not fallback to connlimit default config.")
	}

	snmpInterface = &SNMP{}
	result = snmpInterface.New(map[string]string{"connlimit": "0"})
	if result != nil {
		t.Error("([error] Segment SNMP initiated despide bad config.")
	}
}
