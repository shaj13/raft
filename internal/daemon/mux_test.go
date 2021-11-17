package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testGroupID = uint64(1)

func TestMuxOp(t *testing.T) {
	table := []struct {
		fn func(m *mux)
		ot operationType
	}{
		{
			fn: func(mux *mux) {
				mux.add(testGroupID, nil, nil)
			},
			ot: add,
		},
		{
			fn: func(mux *mux) {
				mux.remove(testGroupID)
			},
			ot: remove,
		},
		{
			fn: func(mux *mux) {
				mux.tick(testGroupID)
			},
			ot: call,
		},
		{
			fn: func(mux *mux) {
				mux.advance(testGroupID)
			},
			ot: advance,
		},
	}

	for _, tt := range table {
		mux := &mux{
			operationc: make(chan *operation),
		}
		go tt.fn(mux)
		op := <-mux.operationc
		close(op.done)
		require.Equal(t, testGroupID, op.gid)
		require.Equal(t, tt.ot, op.ot)
	}
}
