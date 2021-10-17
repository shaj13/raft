package daemon

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInvoke(t *testing.T) {
	temp := make(map[string]int, len(order))
	for k, v := range order {
		temp[k] = v
	}
	defer func() {
		order = temp
	}()

	ctrl := gomock.NewController(t)

	oprs := make([]Operator, 0)
	before := make([]*gomock.Call, 3)
	after := make([]*gomock.Call, 3)

	for i := 0; i < 3; i++ {
		opr := NewMockOperator(ctrl)
		opr.EXPECT().String().Return(fmt.Sprintf("%d", i)).AnyTimes()
		before[i] = opr.EXPECT().before(gomock.Any()).Return(nil)
		after[i] = opr.EXPECT().after(gomock.Any()).Return(nil)
		// append to index 1.
		oprs = append([]Operator{opr}, oprs...)
		// add it to order.
		order[opr.String()] = i
	}

	gomock.InOrder(before...)
	gomock.InOrder(after...)

	// it invoke operator by order.
	err := invoke(nil, oprs...)
	require.NoError(t, err)

	// it return error when operator.before return err.
	opr := NewMockOperator(ctrl)
	opr.EXPECT().before(gomock.Any()).Return(ErrStopped)
	err = invoke(nil, opr)
	require.Equal(t, ErrStopped, err)

	// it return error when operator.after return err.
	opr = NewMockOperator(ctrl)
	opr.EXPECT().before(gomock.Any()).Return(nil)
	opr.EXPECT().after(gomock.Any()).Return(ErrStopped)
	err = invoke(nil, opr)
	require.Equal(t, ErrStopped, err)
}
