package client

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"

	frpnet "github.com/fatedier/frp/pkg/util/net"
)

// net.Conn permits simultaneous Read, Write and Close. HTTP's transport uses
// that contract when a proxy is reloaded with a request still in flight.
type statsTestConn struct{ net.Conn }

func (statsTestConn) Read(p []byte) (int, error)  { return len(p), nil }
func (statsTestConn) Write(p []byte) (int, error) { return len(p), nil }
func (statsTestConn) Close() error                { return nil }

func TestFRPStatsConnConcurrentReadWriteAndClose(t *testing.T) {
	var calls atomic.Int64
	conn := frpnet.WrapStatsConn(statsTestConn{}, func(read, written int64) {
		calls.Add(1)
		if read < 0 || written < 0 {
			t.Error("negative traffic counters")
		}
	})
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for j := 0; j < 10000; j++ {
				_, _ = conn.Read([]byte("request"))
				_, _ = conn.Write([]byte("response"))
				if j == 5000 {
					_ = conn.Close()
				}
			}
		}()
	}
	close(start)
	workers.Wait()
	_ = conn.Close()
	if calls.Load() != 1 {
		t.Fatalf("stats callback called %d times", calls.Load())
	}
}

func TestFRPStatsConnCountsTrafficOnce(t *testing.T) {
	calls := 0
	conn := frpnet.WrapStatsConn(statsTestConn{}, func(read, written int64) {
		calls++
		if read != 7 || written != 8 {
			t.Errorf("wrong totals: read=%d written=%d", read, written)
		}
	})
	_, _ = conn.Read([]byte("request"))
	_, _ = conn.Write([]byte("response"))
	_ = conn.Close()
	_ = conn.Close()
	if calls != 1 {
		t.Fatalf("stats callback called %d times", calls)
	}
}
