package raft

import (
	"bufio"
	"io"

	"github.com/shaj13/raftkit/api"
)

type streamer struct {
	scanner *bufio.Scanner
	index   uint64
}

func (s *streamer) Next() bool {
	return s.scanner.Scan()
}

func (s *streamer) Chunck() *api.Chunk {
	return &api.Chunk{
		Index: s.index,
		Data:  s.scanner.Bytes(),
	}
}

func (s *streamer) Err() error {
	return s.scanner.Err()
}

func (s *streamer) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	n := maxMsgSize - (&api.Chunk{Index: s.index}).Size()
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if len(data) > n {
		return n, data[:n], nil
	}

	return len(data), data, nil
}

type assembler struct {
	writer io.Writer
	pindex uint64
}

func (a *assembler) Write(c *api.Chunk) error {
	if a.pindex == 0 {
		a.pindex = c.Index
	}

	// if a.pindex != c.Index {
	// 	return fmt.Errorf(
	// 		"raft: chunk with index %d is different from the previously received chunk index %d",
	// 		c.Index,
	// 		a.pindex,
	// 	)
	// }

	_, err := a.writer.Write(c.Data)
	return err
}
