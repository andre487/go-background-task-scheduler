package bgscheduler

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.etcd.io/bbolt"
)

type dbWrap struct {
	db           *bbolt.DB
	logger       *logWrap
	queryTimeout time.Duration
	persistent   bool
}

var lastLaunchTimeBucket = []byte("lastLaunchTime")

func newDbWrap(dbPath string, logger *logWrap, queryTimeout time.Duration) (*dbWrap, error) {
	if queryTimeout < 500*time.Millisecond {
		queryTimeout = 500 * time.Millisecond
		logger.Debug("DB query timeout fell back to %s", queryTimeout)
	}
	t := &dbWrap{
		persistent:   false,
		logger:       logger,
		queryTimeout: queryTimeout,
	}
	if dbPath == "" {
		return t, nil
	}

	var err error
	t.db, err = bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: queryTimeout})
	if err != nil {
		return nil, errors.Join(errors.New("unable to create DB connection"), err)
	}

	err = t.initSchema()
	if err != nil {
		return nil, errors.Join(errors.New("unable to init DB schema"), err)
	}

	t.persistent = true
	return t, nil
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
	err := r.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(lastLaunchTimeBucket)
		if bucket == nil {
			return errors.New("nil bucket")
		}
		return bucket.Put([]byte(taskName), []byte(strconv.FormatInt(ts, 10)))
	})
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

	var byteRes []byte
	err := r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(lastLaunchTimeBucket)
		if bucket == nil {
			return errors.New("nil bucket")
		}
		byteRes = bucket.Get([]byte(taskName))
		return nil
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("unable to get last call time from DB for %s", taskName), err)
	}

	if byteRes == nil {
		return &zeroTime, nil
	}

	var ts int64
	ts, err = strconv.ParseInt(string(byteRes), 10, 64)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("unable to convert last call ts for %s", taskName), err)
	}

	tm := time.Unix(ts, 0)
	return &tm, nil
}

func (r *dbWrap) initSchema() error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(lastLaunchTimeBucket)
		return err
	})
}
