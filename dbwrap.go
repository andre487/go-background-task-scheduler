package bgscheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type dbWrap struct {
	Persistent   bool
	db           *sql.DB
	logger       *logWrap
	queryTimeout time.Duration
}

func newDbWrap(dbPath string, logger *logWrap, queryTimeout time.Duration) (*dbWrap, error) {
	if queryTimeout < 500*time.Millisecond {
		queryTimeout = 500 * time.Millisecond
	}
	r := &dbWrap{
		Persistent:   false,
		logger:       logger,
		queryTimeout: queryTimeout,
	}
	if dbPath == "" {
		return r, nil
	}

	var err error
	r.db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.Join(errors.New("unable to create DB connection"), err)
	}
	r.Persistent = true

	err = r.initSchema()
	if err != nil {
		return nil, errors.Join(errors.New("unable to init DB schema"), err)
	}

	return r, nil
}

func (r *dbWrap) SetLastLaunch(taskName string, lastLaunch time.Time) error {
	ts := lastLaunch.Unix()

	ctx, cancel := r.context()
	defer cancel()

	_, err := r.db.ExecContext(ctx, "REPLACE INTO LastLaunches (TaskName, Ts) VALUES (?, ?)", taskName, ts)
	if err != nil {
		return errors.Join(fmt.Errorf("unable to set last call time for %s", taskName), err)
	}

	return nil
}

func (r *dbWrap) GetLastLaunch(taskName string) (*time.Time, error) {
	ctx, cancel := r.context()
	defer cancel()

	res := r.db.QueryRowContext(ctx, "SELECT Ts FROM LastLaunches WHERE TaskName=?", taskName)
	if err := res.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("unable to get last call time for %s", taskName), err)
	}

	var ts int64
	if err := res.Scan(&ts); err != nil {
		return nil, errors.Join(fmt.Errorf("unable to get last call time for %s", taskName), err)
	}

	tm := time.Unix(ts, 0)
	return &tm, nil
}

func (r *dbWrap) initSchema() error {
	if !r.Persistent {
		return nil
	}
	initQueries := []string{
		`
		CREATE TABLE IF NOT EXISTS LastLaunches (
		    TaskName TEXT NOT NULL PRIMARY KEY,
		    Ts INTEGER NOT NULL
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS ExactTimeConfigs (
			TaskName TEXT NOT NULL PRIMARY KEY,
		    Hour INTEGER NOT NULL,
		    Minute INTEGER NOT NULL,
		    Second INTEGER NOT NULL
		)
		`,
		`CREATE INDEX IF NOT EXISTS ExecTime ON ExactTimeConfigs (Hour, Minute, Second)`,
	}

	ctx, cancel := r.context()
	defer cancel()

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	for _, query := range initQueries {
		_, err := tx.Exec(query)
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return errors.Join(err, rbErr)
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return nil
}

func (r *dbWrap) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.queryTimeout)
}
