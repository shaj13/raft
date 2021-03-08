package raft

import (
	"fmt"

	"go.etcd.io/etcd/raft/v3/raftpb"
)

const maxMsgSize = 4 << 20 // TODO: read me from cfg

func chunkedMsg(m raftpb.Message) []raftpb.Message {
	if m.Size() > maxMsgSize && m.Type == raftpb.MsgSnap {
		return splitSnap(&m)
	}

	if m.Size() > maxMsgSize {
		return splitEntries(&m)
	}

	return []raftpb.Message{m}
}

func assembleEntries(msgs []raftpb.Message) (*raftpb.Message, error) {
	var msg *raftpb.Message

	if len(msgs) == 0 {
		return nil, fmt.Errorf("expected an msg but recived 0")
	}

	for _, m := range msgs {
		if msg == nil {
			recvd := m
			msg = &recvd
			continue
		}

		if msg.Index != m.Index {
			return nil, fmt.Errorf(
				"raftkit: Message chunk with index %d is different from the previously received raft message index %d",
				m.Index,
				msg.Index,
			)
		}

		// we should not continue but for now lets accept it.
		if len(m.Entries) == 0 {
			continue
		}

		ent := msg.Entries[len(msg.Entries)-1]

		if ent.Index != m.Entries[0].Index {
			msg.Entries = append(msg.Entries, m.Entries[0])
			continue
		}

		(&ent).Data = append(ent.Data, m.Entries[0].Data...)
		msg.Entries[len(msg.Entries)-1] = ent
	}
	return msg, nil
}

func assembleSnap(msgs []raftpb.Message) (*raftpb.Message, error) {
	var msg *raftpb.Message
	if len(msgs) == 0 {
		return nil, fmt.Errorf("expected an msg but recived 0")
	}

	for _, m := range msgs {
		if msg == nil {
			msg = &m
			continue
		}

		if msg.Index != m.Index {
			return nil, fmt.Errorf(
				"raftkit: message chunk with index %d is different from the previously received raft message index %d",
				m.Index,
				msg.Index,
			)
		}

		msg.Snapshot.Data = append(msg.Snapshot.Data, m.Snapshot.Data...)
	}
	return nil, nil
}

func splitEntries(m *raftpb.Message) (msgs []raftpb.Message) {
	for _, e := range m.Entries {
		n := maxMsgSize - (m.Size() - (e.Size() - len(e.Data)))
		chunks := split(n, m.Snapshot.Data)
		for _, data := range chunks {
			ch := *m
			ce := e
			(&ce).Data = data
			ch.Entries = []raftpb.Entry{ce}
			msgs = append(msgs, ch)
		}
	}
	return msgs
}

func splitSnap(m *raftpb.Message) (msgs []raftpb.Message) {
	n := maxMsgSize - (m.Size() - len(m.Snapshot.Data))
	chunks := split(n, m.Snapshot.Data)
	for _, data := range chunks {
		ch := *m
		ch.Snapshot.Data = data
		msgs = append(msgs, ch)
	}
	return msgs
}

func split(n int, data []byte) (chunks [][]byte) {
	size := len(data)
	for index := 0; index < size; {
		chunckSize := index + n
		if chunckSize > size {
			chunckSize = size
		}
		chunck := data[index:chunckSize]
		index = chunckSize
		chunks = append(chunks, chunck)
	}
	return chunks
}
