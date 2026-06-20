package event

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type countingSub struct{ n int64 }

func (c *countingSub) Handle(_ Event) { atomic.AddInt64(&c.n, 1) }

type panicSub struct{}

func (panicSub) Handle(_ Event) { panic("subscriber boom") }

func discardEntry() *logrus.Entry {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return logrus.NewEntry(l)
}

// OBS-1: synchronous subscribers receive EVERY event in a burst — none dropped.
func TestEmitter_SyncBurst_NoLoss(t *testing.T) {
	e := NewEmitter()
	defer e.Close()
	c := &countingSub{}
	e.Subscribe(c)

	const n = 2000 // far exceeds the async channel buffer (256)
	for i := 0; i < n; i++ {
		e.Publish(Event{Type: TypeEmployeeCreated, TenantID: uuid.New()})
	}
	assert.Equal(t, int64(n), atomic.LoadInt64(&c.n), "no synchronous (audit) events may be dropped under burst")
}

// OBS-1: a panicking synchronous subscriber must not crash the publisher, and
// later subscribers/events must still be processed.
func TestEmitter_SyncPanicIsolated(t *testing.T) {
	e := NewEmitter()
	defer e.Close()
	e.SetLogger(discardEntry())
	c := &countingSub{}
	e.Subscribe(panicSub{})
	e.Subscribe(c) // registered after the panicking one

	assert.NotPanics(t, func() {
		e.Publish(Event{Type: TypeEmployeeCreated})
		e.Publish(Event{Type: TypeEmployeeCreated})
	})
	assert.Equal(t, int64(2), atomic.LoadInt64(&c.n), "subscriber after a panicking one must still run")
}

// OBS-1: a panicking ASYNC subscriber must not kill the worker goroutine.
func TestEmitter_AsyncPanicIsolated(t *testing.T) {
	e := NewEmitter()
	e.SetLogger(discardEntry())

	var wg sync.WaitGroup
	wg.Add(3)
	good := &waitingSub{wg: &wg}
	e.SubscribeAsync(panicSub{})
	e.SubscribeAsync(good)

	for i := 0; i < 3; i++ {
		e.Publish(Event{Type: TypeEmployeeCreated})
	}
	wg.Wait() // would hang/fail if the worker died after the first panic
	e.Close()
	assert.Equal(t, int64(3), atomic.LoadInt64(&good.n))
}

type waitingSub struct {
	wg *sync.WaitGroup
	n  int64
}

func (w *waitingSub) Handle(_ Event) { atomic.AddInt64(&w.n, 1); w.wg.Done() }

// OBS-1: a failing audit DB write is handled (logged), not swallowed silently,
// and never panics.
func TestAuditSubscriber_DBErrorIsHandled(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	mock.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnError(errors.New("db down"))

	sub := NewAuditSubscriber(gdb, discardEntry())
	emp := &model.Employee{Name: "X"}
	emp.ID = uuid.New()

	assert.NotPanics(t, func() {
		sub.Handle(Event{Type: TypeEmployeeCreated, TenantID: uuid.New(), Payload: emp})
	})
	assert.NoError(t, mock.ExpectationsWereMet())
}
