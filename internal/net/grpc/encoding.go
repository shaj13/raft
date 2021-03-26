package grpc

import (
	"bufio"
	"fmt"
	"io"

	"github.com/shaj13/raftkit/api"
)

func newEncoder(r io.Reader) *encoder {
	e := new(encoder)
	e.scanner = bufio.NewScanner(r)
	e.scanner.Split(e.scan)
	return e
}

func newDecoder(w io.Writer) *decoder {
	return &decoder{
		w: w,
	}
}

type encoder struct {
	scanner *bufio.Scanner
	err     error
	index   uint64
}

func (e *encoder) Encode(cb func(*api.Chunk) error) error {
	for e.scanner.Scan() {
		if err := cb(e.chunck()); err != nil {
			return err
		}
	}
	return e.scanner.Err()
}

func (e *encoder) chunck() *api.Chunk {
	defer func() {
		e.index++
	}()
	return &api.Chunk{
		Index: e.index,
		Data:  e.scanner.Bytes(),
	}
}

func (e *encoder) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	n := bufio.MaxScanTokenSize - (&api.Chunk{Index: e.index}).Size()
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if len(data) > n {
		return n, data[:n], nil
	}

	return len(data), data, nil
}

type decoder struct {
	w     io.Writer
	index uint64
}

func (d *decoder) Decode(c *api.Chunk) error {
	defer func() {
		d.index++
	}()

	if d.index != c.Index {
		return fmt.Errorf(
			"raft/net/grpc: index mismatch, chunk with index %d is different from the expected index %d",
			c.Index,
			d.index,
		)
	}

	_, err := d.w.Write(c.Data)
	return err
}
