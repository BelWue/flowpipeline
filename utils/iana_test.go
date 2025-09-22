package utils

import (
	"testing"
)

func TestIanaProtocolNumberToName(t *testing.T) {
	tests := []struct {
		protoNum uint32
		name     string
	}{
		{
			protoNum: 6,
			name:     "TCP",
		},
		{
			// empty name field, should yield description
			protoNum: 68,
			name:     "any distributed file system",
		},
		{
			// not included in IANA list
			protoNum: 222,
			name:     "",
		},
		{
			// out of spec (>256)
			protoNum: 12345,
			name:     "",
		},
	}

	for _, test := range tests {
		if IanaProtocolNumberToName(test.protoNum) != test.name {
			t.Errorf("IanaProtocolNumberToName(%d) = %s, expected %s",
				test.protoNum,
				IanaProtocolNumberToName(test.protoNum),
				test.name)
		}
	}
}

func TestIanaProtocolNumberToLowercaseName(t *testing.T) {
	tests := []struct {
		protoNum uint32
		name     string
	}{
		{
			protoNum: 6,
			name:     "tcp",
		},
		{
			// empty name field, should yield description
			protoNum: 68,
			name:     "any distributed file system",
		},
		{
			// not included in IANA list
			protoNum: 222,
			name:     "",
		},
		{
			// out of spec bounds
			protoNum: 12345,
			name:     "",
		},
	}

	for _, test := range tests {
		if name := IanaProtocolNumberToLowercaseName(test.protoNum); name != test.name {
			t.Errorf("IanaProtocolNumberToLowercaseName(%d) = %s, expected %s",
				test.protoNum,
				name,
				test.name)
		}
	}
}

func BenchmarkIanaProtocolNumberToName(b *testing.B) {
	for b.Loop() {
		IanaProtocolNumberToName(uint32(b.N))
	}
}
