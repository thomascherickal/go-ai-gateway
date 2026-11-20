package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
)

type stubStep struct {
	match        string
	rowsAffected int64
	columns      []string
	rows         [][]driver.Value
	err          error
	rowsErr      error
}

type errRowsResult struct{ err error }

func (e errRowsResult) LastInsertId() (int64, error) { return 0, e.err }
func (e errRowsResult) RowsAffected() (int64, error) { return 0, e.err }

type stubConnector struct {
	steps []stubStep
	idx   int
	calls []string
}

func (c *stubConnector) Connect(context.Context) (driver.Conn, error) { return c, nil }
func (c *stubConnector) Driver() driver.Driver                        { return nil }
func (c *stubConnector) Prepare(string) (driver.Stmt, error)          { return nil, errors.New("unused") }
func (c *stubConnector) Close() error                                 { return nil }
func (c *stubConnector) Begin() (driver.Tx, error)                    { return nil, errors.New("no tx") }

func (c *stubConnector) nextStep(query string) (stubStep, error) {
	c.calls = append(c.calls, query)
	if c.idx >= len(c.steps) {
		return stubStep{}, errors.New("unexpected SQL call: " + query)
	}
	st := c.steps[c.idx]
	c.idx++
	if st.match != "" && !strings.Contains(query, st.match) {
		return stubStep{}, errors.New("SQL mismatch: got " + query + " want substring " + st.match)
	}
	return st, nil
}

func (c *stubConnector) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	st, err := c.nextStep(query)
	if err != nil {
		return nil, err
	}
	if st.err != nil {
		return nil, st.err
	}
	if st.rowsErr != nil {
		return errRowsResult{err: st.rowsErr}, nil
	}
	return driver.RowsAffected(st.rowsAffected), nil
}

func (c *stubConnector) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	st, err := c.nextStep(query)
	if err != nil {
		return nil, err
	}
	if st.err != nil {
		return nil, st.err
	}
	return &stubRows{cols: st.columns, data: st.rows}, nil
}

type stubRows struct {
	cols []string
	data [][]driver.Value
	pos  int
}

func (r *stubRows) Columns() []string { return r.cols }
func (r *stubRows) Close() error      { return nil }
func (r *stubRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}
