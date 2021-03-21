package raft

import "github.com/gogo/protobuf/proto"

func mustMarshal(m proto.Marshaler) []byte {
	data, err := m.Marshal()
	if err != nil {
		panic(err)
	}
	return data
}

func mustUnmarshal(data []byte, m proto.Unmarshaler) {
	if err := m.Unmarshal(data); err != nil {
		panic(err)
	}
}
