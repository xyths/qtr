package executor

import (
	"context"
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/trader/rest"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"sync"
	"time"
)

type ClientIdManager struct {
	sep    string
	lock   sync.RWMutex
	long   int64
	short  int64
	unique int64
	coll   *mongo.Collection
}

func (m *ClientIdManager) Init(sep string, coll *mongo.Collection) {
	m.sep = sep
	m.coll = coll
}

func (m *ClientIdManager) Load(ctx context.Context) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.long, err = hs.LoadInt64(ctx, m.coll, "long"); err != nil {
		return
	}
	if m.short, err = hs.LoadInt64(ctx, m.coll, "short"); err != nil {
		return
	}
	if m.unique, err = hs.LoadInt64(ctx, m.coll, "unique"); err != nil {
		return
	}
	return
}

func (m *ClientIdManager) LongAdd(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.long++
	return hs.SaveInt64(ctx, m.coll, "long", m.long)
}

func (m *ClientIdManager) ShortAdd(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.short++
	return hs.SaveInt64(ctx, m.coll, "short", m.short)
}
func (m *ClientIdManager) LongReset(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.long = 0
	return hs.SaveInt64(ctx, m.coll, "long", m.long)
}

func (m *ClientIdManager) ShortReset(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.short = 0
	return hs.SaveInt64(ctx, m.coll, "short", m.short)
}

func (m *ClientIdManager) GetClientOrderId(ctx context.Context, prefix string) (string, error) {
	unique, err := m.getUniqueId(ctx)
	if err != nil {
		return "", err
	}
	m.lock.RLock()
	short := m.short
	long := m.long
	m.lock.RUnlock()
	return fmt.Sprintf("%[2]s%[1]s%[3]d%[1]s%[4]d%[1]s%[5]d", m.sep, prefix, short, long, unique), nil
}
func (m *ClientIdManager) getUniqueId(ctx context.Context, ) (int64, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.unique = (m.unique + 1) % 10000
	if err := hs.SaveInt64(ctx, m.coll, "uniqueId", m.unique); err != nil {
		return 0, errors.New(fmt.Sprintf("save uniqueId error: %s", err))
	}
	return m.unique, nil
}

type OrderProxy struct {
	coll *mongo.Collection
}

func (p *OrderProxy) Init(coll *mongo.Collection) {
	p.coll = coll
}

func (p *OrderProxy) AddOrder(ctx context.Context, o rest.Order) error {
	option := options.FindOneAndUpdate().SetUpsert(true)
	r := p.coll.FindOneAndUpdate(
		ctx,
		bson.D{
			{"_id", o.Id},
		},
		bson.D{
			{"$set", bson.D{
				{"clientOrderId", o.ClientOrderId},
				{"total", o.Total},
				{"updated", o.Updated},
			}},
		},
		option,
	)

	if r.Err() != nil {
		return errors.New(fmt.Sprintf("add order error: %s", r.Err()))
	}
	return nil
}

func (p *OrderProxy) CreateOrder(ctx context.Context, o rest.Order) error {
	option := options.FindOneAndUpdate().SetUpsert(true)
	r := p.coll.FindOneAndUpdate(
		ctx,
		bson.D{
			{"_id", o.Id},
		},
		bson.D{
			{"$set", bson.D{
				{"clientOrderId", o.ClientOrderId},
				{"type", o.Type},
				{"price", o.Price},
				{"amount", o.Amount},
				{"total", o.Total},
				{"status", o.Status},
				{"updated", o.Updated},
			}},
		},
		option,
	)
	if r.Err() != nil {
		return errors.New(fmt.Sprintf("create order error: %s", r.Err()))
	}
	return nil
}

func (p *OrderProxy) FillOrder(ctx context.Context, o rest.Order, t rest.Trade) error {
	option := options.FindOneAndUpdate().SetUpsert(true)
	r := p.coll.FindOneAndUpdate(
		ctx,
		bson.D{
			{"_id", o.Id},
		},
		bson.D{
			{"$push", bson.D{
				{"trades", t},
			}},
			{"$set", bson.D{
				{"status", o.Status},
				{"updated", time.Now()},
			}},
		},
		option,
	)
	if r.Err() != nil {
		return errors.New(fmt.Sprintf("fill order error: %s", r.Err()))
	}
	return nil
}

func (p *OrderProxy) GetOrder(ctx context.Context, o *rest.Order) error {
	return p.coll.FindOne(ctx, bson.M{
		"_id": o.Id,
	}).Decode(o)
}

type Quota struct {
	lock  sync.RWMutex
	quota decimal.Decimal
	coll  *mongo.Collection
}

func (q *Quota) Init(coll *mongo.Collection, initQuota decimal.Decimal) {
	q.coll = coll
	q.quota = initQuota
}

func (q *Quota) Load(ctx context.Context) (err error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	var str string
	if err = hs.LoadKey(ctx, q.coll, "quota", &str); err != nil {
		return
	}
	if quota, err1 := decimal.NewFromString(str); err1 != nil {
		return err1
	} else {
		q.quota = quota
	}

	return nil
}

func (q *Quota) Get() decimal.Decimal {
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.quota
}

func (q *Quota) Add(quota decimal.Decimal) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.quota = q.quota.Add(quota)
	q.save()
}

func (q *Quota) Sub(quota decimal.Decimal) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.quota = q.quota.Sub(quota)
	q.save()
}

func (q *Quota) save() {
	if err := hs.SaveKey(context.Background(), q.coll, "quota", q.quota.String()); err != nil {
		log.Printf("save quota error: %s", err)
	}
}
