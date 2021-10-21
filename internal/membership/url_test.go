package membership

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURL(t *testing.T) {
	table := []struct {
		id   uint64
		addr string
		raw  string
		err  string
	}{
		{id: 1, addr: ":80", raw: URL(1, ":80")},
		{err: "invalid", raw: ":80"},
		{err: "invalid", raw: "sa=:80"},
		{err: "invalid", raw: "=:80"},
		{err: "zero", raw: "0=:80"},
	}

	for _, tt := range table {
		id, addr, err := ParseURL(tt.raw)
		if err != nil {
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.Equal(t, tt.id, id)
			require.Equal(t, tt.addr, addr)
		}
	}
}
