package context

import (
	"context"
	"database/sql"
	"fmt"

	"d8.io/upmeter/pkg/util"
)

const (
	defaultPoolSize = 20
)

type DbContext struct {
	conns *pool
	db    *sql.DB
	tx    *sql.Tx
}

func NewDbContext() *DbContext {
	return &DbContext{}
}

// Connect opens a DB without pooling. Mainly for tests.
func (c *DbContext) Connect(path string) error {
	db, err := open(path, nil)
	if err != nil {
		return err
	}
	c.db = db
	return nil
}

// ConnectWithPool creates a pool of DB connections.
func (c *DbContext) ConnectWithPool(path string, opts map[string]string) error {
	size := util.GetenvInt64("UPMETER_DB_POOL_SIZE")
	if size == 0 {
		size = defaultPoolSize
	}
	c.conns = newPool(size)

	return c.conns.Connect(path, opts)
}

func (c *DbContext) Handler() *sql.DB {
	return c.db
}

func (c *DbContext) Copy() *DbContext {
	return &DbContext{
		conns: c.conns,
		db:    c.db,
		tx:    c.tx,
	}
}

func (c *DbContext) StmtRunner() StmtRunner {
	if c.tx != nil {
		return c.tx
	}

	if c.db != nil {
		return c.db
	}

	panic("Call StmtRunner from uninitialized DbContext")
}

// Start captures a connection from pool and returns a stoppable context.
// If context is stoppable, returns non-stoppable db-only context.
func (c *DbContext) Start() *DbContext {
	if c.tx != nil {
		return &DbContext{tx: c.tx}
	}

	// Do not copy pool if the db is already captured.
	if c.db != nil {
		return &DbContext{db: c.db}
	}

	// Capture connection from the pool if it is a "root" context.
	if c.conns != nil && c.db == nil {
		db := c.conns.Capture()
		return &DbContext{
			conns: c.conns,
			db:    db,
		}
	}

	panic("Call Start from uninitialized DbContext")
}

func (c *DbContext) Stop() {
	if c.conns != nil && c.db != nil {
		c.conns.Release(c.db)
	}
}

// BeginTransaction starts a transaction with default driver options: the isolation level and the readonly flag.
func (c *DbContext) BeginTransaction() (*DbContext, error) {
	ctx := context.Background() // FIXME (e.shevchenko) pass the context from the outside

	if c.tx != nil {
		return &DbContext{tx: c.tx}, nil
	}

	if c.db == nil {
		return nil, fmt.Errorf("begin transaction from uninitialized DbContext")
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &DbContext{tx: tx}, nil
}

func (c *DbContext) Rollback() error {
	if c.tx != nil {
		return c.tx.Rollback()
	}
	return nil
}

func (c *DbContext) Commit() error {
	if c.tx != nil {
		return c.tx.Commit()
	}
	return nil
}