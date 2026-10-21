package workerd

import (
	"github.com/Sakurame1/frp-manager/services/app"
	"testing"
)

type testWorker struct{ stopped int }

func (w *testWorker) RunWorker(*app.Context)  {}
func (w *testWorker) StopWorker(*app.Context) { w.stopped++ }
func (w *testWorker) GarbageCollect()         {}
func (w *testWorker) Init(*app.Context) error { return nil }

func TestAuthoritativeSyncStopsRevokedWorkers(t *testing.T) {
	m := NewWorkersManager()
	keep, revoked := &testWorker{}, &testWorker{}
	m.workers.Store("keep", keep)
	m.workers.Store("revoked", revoked)
	ctx := app.NewContext(nil, app.NewApp())
	m.RetainWorkers(ctx, map[string]bool{"keep": true})
	if keep.stopped != 0 || revoked.stopped != 1 {
		t.Fatal("wrong worker stopped")
	}
	if _, ok := m.GetWorker(ctx, "revoked"); ok {
		t.Fatal("revoked worker retained")
	}
	m.RetainWorkers(ctx, map[string]bool{})
	if keep.stopped != 1 {
		t.Fatal("empty successful sync retained running worker")
	}
}
