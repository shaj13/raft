package msgbus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMsgBus(t *testing.T) {
	eventid := uint64(1)
	onceid := uint64(2)

	val := "some value"
	m := New()
	s1 := m.Subscribe(eventid)
	s2 := m.SubscribeOnce(onceid)
	defer m.Close()

	run := func(id uint64, s1, s2 *Subscription) bool {
		go func() {
			m.Broadcast(id, val)
		}()

		select {
		case v, ok := <-s1.Chan():
			if ok {
				assert.Equal(t, val, v)
				return true
			}
			return false
		case <-s2.Chan():
			t.Fatal("unexpected broadcast call")
		case <-time.After(time.Millisecond * 500):
			return false
		}

		return false
	}

	unsubscribe := func(s *Subscription) {
		s.Unsubscribe()
		assert.Equal(t, len(m.events[s.eid]), 0)
		assert.Equal(t, len(m.events), 1)
	}

	// Round #1 check normal subscribe
	// caled more than once.
	for i := 0; i < 2; i++ {
		ok := run(eventid, s1, s2)
		assert.True(t, ok, "subscribe chan not called")
	}

	unsubscribe(s1)

	// Round #2 check once subscribe
	// caled only once.
	s1 = m.Subscribe(eventid)
	count := 0
	for i := 0; i < 2; i++ {
		if ok := run(onceid, s2, s1); ok {
			count++
		}
	}

	assert.Equalf(
		t,
		1,
		count,
		"subscribe once chan not called or called mor than once, Count %d",
		count,
	)

	unsubscribe(s2)
}
