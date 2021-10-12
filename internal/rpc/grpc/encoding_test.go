package grpc

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/assert"
)

func TestDecoder(t *testing.T) {
	w := new(bytes.Buffer)
	dec := newDecoder(w)

	// Round #1 it retrun error when index mismatch
	dec.index = 2
	err := dec.Decode(&raftpb.Chunk{})
	assert.Contains(t, err.Error(), "index mismatch")

	// Round #2 it assemble all chunks
	dec.index = 0
	for i, str := range strings.Split("Test Decoder", " ") {
		err = dec.Decode(&raftpb.Chunk{Index: uint64(i), Data: []byte(str)})
		assert.NoError(t, err)
	}

	// space removed by split func.
	assert.Equal(t, "TestDecoder", w.String())
}

func TestEncoder(t *testing.T) {
	count := 0
	cb := func(c *raftpb.Chunk) error {
		count++
		assert.LessOrEqual(t, c.Size(), bufio.MaxScanTokenSize)
		return nil
	}

	table := []struct {
		name  string
		r     io.Reader
		count int
	}{
		{
			name:  "it retrun one chunk when msg size less than max",
			r:     bytes.NewBufferString("TestEncoderErr"),
			count: 1,
		},
		{
			name:  "it split msg int chunks",
			r:     io.LimitReader(rand.Reader, bufio.MaxScanTokenSize*10),
			count: 160,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			count = 0
			enc := newEncoder(tt.r)
			err := enc.Encode(cb)
			assert.NoError(t, err)
			assert.Equal(t, tt.count, count)
		})
	}
}

func TestEncodingCompatibility(t *testing.T) {
	msg := make([]byte, bufio.MaxScanTokenSize*10)
	_, err := rand.Reader.Read(msg)
	if err != nil {
		t.Fatal(err)
	}

	w := new(bytes.Buffer)
	enc := newEncoder(bytes.NewBuffer(msg))
	dec := newDecoder(w)

	err = enc.Encode(func(c *raftpb.Chunk) error {
		return dec.Decode(c)
	})

	assert.NoError(t, err)
	assert.Equal(t, msg, w.Bytes())
}

func TestEncoderErr(t *testing.T) {
	r := bytes.NewBufferString("TestEncoderErr")
	cerr := fmt.Errorf("TestEncoderErr cb error")
	enc := newEncoder(r)
	err := enc.Encode(func(c *raftpb.Chunk) error { return cerr })
	assert.Equal(t, cerr, err)
}
