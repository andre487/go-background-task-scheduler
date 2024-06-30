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
	db           *sql.DB
	logger       *logWrap
	queryTimeout time.Duration
	persistent   bool

	setLastLaunchQuery      *sql.Stmt
	getLastLaunchQuery      *sql.Stmt
	setExactTimeConfigQuery *sql.Stmt
	getExactTimeConfigQuery *sql.Stmt
}

func newDbWrap(dbPath string, logger *logWrap, queryTimeout time.Duration) (*dbWrap, error) {
	if queryTimeout < 500*time.Millisecond {
		queryTimeout = 500 * time.Millisecond
		logger.Debug("DB query timeout fell back to %s", queryTimeout)
	}
	r := &dbWrap{
		persistent:   false,
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
	r.persistent = true

	err = r.initSchema()
	if err != nil {
		return nil, errors.Join(errors.New("unable to init DB schema"), err)
	}

	err = r.prepareQueries()
	if err != nil {
		return nil, errors.Join(errors.New("unable to prepare DB queries"), err)
	}

	return r, nil
}

func (r *dbWrap) Close() {
	if r.db == nil {
		return
	}
	err := r.db.Close()
	if err != nil {
		r.logger.Warn("Error when closing DB: %s", err)
	}
}

func (r *dbWrap) Persistent() bool {
	return r.persistent
}

func (r *dbWrap) SetLastLaunch(taskName string, lastLaunch time.Time) error {
	if !r.persistent {
		r.logger.Warn("DB is not persistent, not exec SetLastLaunch")
		return nil
	}

	ts := lastLaunch.Unix()

	ctx, cancel := r.context()
	defer cancel()

	_, err := r.setLastLaunchQuery.ExecContext(ctx, taskName, ts)
	if err != nil {
		return errors.Join(fmt.Errorf("unable to set last call time for %s", taskName), err)
	}

	return nil
}

func (r *dbWrap) GetLastLaunch(taskName string) (*time.Time, error) {
	if !r.persistent {
		r.logger.Warn("DB is not persistent, not exec GetLastLaunch")
		return &zeroTime, nil
	}

	ctx, cancel := r.context()
	defer cancel()

	res := r.getLastLaunchQuery.QueryRowContext(ctx, taskName)
	if err := res.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("unable to get last call time for %s", taskName), err)
	}

	var ts int64
	if err := res.Scan(&ts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &zeroTime, nil
		}
		return nil, errors.Join(fmt.Errorf("unable to get last call time for %s", taskName), err)
	}

	tm := time.Unix(ts, 0)
	return &tm, nil
}

func (r *dbWrap) SetExactTimeConfig(taskName string, tm ExactLaunchTime) error {
	if !r.persistent {
		r.logger.Warn("DB is not persistent, not exec SetExactTimeConfig")
		return nil
	}

	ctx, cancel := r.context()
	defer cancel()

	_, err := r.setExactTimeConfigQuery.ExecContext(ctx, taskName, tm.Hour, tm.Minute, tm.Second)
	if err != nil {
		return errors.Join(fmt.Errorf("unable to set exact time config for %s", taskName), err)
	}

	return nil
}

func (r *dbWrap) GetExactTimeConfig(taskName string) (*ExactLaunchTime, error) {
	if !r.persistent {
		r.logger.Warn("DB is not persistent, not exec GetExactTimeConfig")
		return nil, nil
	}

	ctx, cancel := r.context()
	defer cancel()

	res := r.getExactTimeConfigQuery.QueryRowContext(ctx, taskName)
	if err := res.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("unable to get exact time config for %s", taskName), err)
	}

	var tm ExactLaunchTime
	if err := res.Scan(&tm.Hour, &tm.Minute, &tm.Second); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errors.Join(fmt.Errorf("unable to get exact time config for %s", taskName), err)
	}
	return &tm, nil
}

func (r *dbWrap) initSchema() error {
	if !r.persistent {
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

func (r *dbWrap) prepareQueries() error {
	if !r.persistent {
		return nil
	}

	ctx, cancel := r.context()
	defer cancel()

	var st *sql.Stmt
	var err error

	st, err = r.db.PrepareContext(ctx, "REPLACE INTO LastLaunches (TaskName, Ts) VALUES (?, ?)")
	if err != nil {
		return err
	}
	r.setLastLaunchQuery = st

	st, err = r.db.PrepareContext(ctx, "SELECT Ts FROM LastLaunches WHERE TaskName=?")
	if err != nil {
		return err
	}
	r.getLastLaunchQuery = st

	st, err = r.db.PrepareContext(ctx, "REPLACE INTO ExactTimeConfigs (TaskName, Hour, Minute, Second) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	r.setExactTimeConfigQuery = st

	st, err = r.db.PrepareContext(ctx, "SELECT Hour, Minute, Second FROM ExactTimeConfigs WHERE TaskName=?")
	if err != nil {
		return err
	}
	r.getExactTimeConfigQuery = st

	return nil
}

func (r *dbWrap) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.queryTimeout)
}
