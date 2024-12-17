package pb

import (
	"bytes"

	"google.golang.org/protobuf/encoding/protodelim"
)

type ProtoProducerMessage struct {
	EnrichedFlow
}

func (m *ProtoProducerMessage) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	_, err := protodelim.MarshalTo(buf, m)
	return buf.Bytes(), err
}
