// Package db provides persistent storage implementation using a PostgreSQL database.
// It handles database connection, schema migration, and bulk operations for metrics.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"gometrics/internal/api/metricsdto"

	_ "github.com/lib/pq"
)

// initDDL contains the SQL statement to create the initial schema for metrics.
const initDDL = `
CREATE TABLE IF NOT EXISTS metrics (
    ID      TEXT PRIMARY KEY,
    MType   TEXT NOT NULL,
    Delta   BIGINT,
    Value   DOUBLE PRECISION,
    UpdateAt TIMESTAMPTZ DEFAULT now()
);
`

// DBStorage represents a storage implementation backed by a SQL database.
// It embeds *sql.DB to provide direct access to database operations if needed.
type DBStorage struct {
	*sql.DB
	storeInter int
}

// CreateConnection establishes a connection to the database and applies initial migrations.
// It returns a new *DBStorage instance or an error if connection or migration fails.
//
// The 'connectionString' parameter typically follows the PostgreSQL DSN format.
func CreateConnection(ctx context.Context, dbType, connectionString string) (*DBStorage, error) {
	db, err := sql.Open(dbType, connectionString)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("rollback failed: %v (original err: %w)", rbErr, err)
			}
		}
	}()

	if _, err := db.ExecContext(ctx, initDDL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return &DBStorage{db, 0}, tx.Commit()
}

// Ping checks the connection to the database.
// It returns an error if the database is unreachable.
func (db *DBStorage) Ping(ctx context.Context) error {
	return db.PingContext(ctx)
}

// ImportLogs retrieves all metrics currently stored in the database.
// It returns a slice of metricsdto.Metrics or an error.
func (db *DBStorage) ImportLogs(ctx context.Context) ([]metricsdto.Metrics, error) {
	metrics := make([]metricsdto.Metrics, 0)

	rows, err := db.QueryContext(ctx, "SELECT ID, MType, Delta, Value from metrics")
	if err != nil {
		return nil, err
	}
	// Always close rows to release the connection.
	defer rows.Close()

	var (
		delta sql.NullInt64
		value sql.NullFloat64
	)

	for rows.Next() {
		var v metricsdto.Metrics
		err = rows.Scan(&v.ID, &v.MType, &delta, &value)
		if err != nil {
			return nil, err
		}

		if delta.Valid {
			d := delta.Int64
			v.Delta = &d
		}
		if value.Valid {
			val := value.Float64
			v.Value = &val
		}

		metrics = append(metrics, v)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return metrics, nil
}

// FormattingLogs bulk inserts or updates the provided gauge and counter metrics.
// It uses a transaction to ensure atomicity.
//
// For each metric, it performs an "UPSERT" operation:
// - If the metric ID exists, it updates the value/delta and sets UpdateAt to now().
// - If it does not exist, it inserts a new row.
func (db *DBStorage) FormattingLogs(ctx context.Context, gauge map[string]float64, counter map[string]int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("rollback failed: %v (original err: %w)", rbErr, err)
			}
		}
	}()

	gaugeStmt, err := tx.PrepareContext(ctx, `
        INSERT INTO metrics (ID, MType, Delta, Value)
        VALUES ($1, 'gauge', NULL, $2)
        ON CONFLICT (id) DO UPDATE
        SET value = EXCLUDED.value, delta = NULL, UpdateAt = now();
    `)
	if err != nil {
		return err
	}
	defer gaugeStmt.Close()

	counterStmt, err := tx.PrepareContext(ctx, `
        INSERT INTO metrics (ID, MType, Delta, Value)
        VALUES ($1, 'counter', $2, NULL)
        ON CONFLICT (id) DO UPDATE
        SET delta = EXCLUDED.delta, value = NULL, UpdateAt = now();
    `)
	if err != nil {
		return err
	}
	defer counterStmt.Close()

	for gkey, gvalue := range gauge {
		_, err := gaugeStmt.ExecContext(ctx, gkey, gvalue)
		if err != nil {
			return fmt.Errorf("cannot insert gauge: %v", err)
		}
	}
	for ckey, cvalue := range counter {
		_, err := counterStmt.ExecContext(ctx, ckey, int64(cvalue))
		if err != nil {
			return fmt.Errorf("cannot insert counter: %v", err)
		}
	}

	return tx.Commit()
}

// GetLoopTime returns the configured storage interval (currently always 0 for DB).
func (db *DBStorage) GetLoopTime() int {
	return db.storeInter
}

// Flush is a no-op for DBStorage as data is persisted immediately.
func (db *DBStorage) Flush() error {
	return nil
}
