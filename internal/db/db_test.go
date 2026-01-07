package db

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestCreateConnection verifies the initialization sequence: Open -> Ping -> Begin -> Exec DDL -> Commit.
func TestCreateConnection(t *testing.T) {
	const dsn = "sqlmock_create_conn"

	sqlDB, mock, err := sqlmock.NewWithDSN(dsn, sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	mock.ExpectPing()

	mock.ExpectBegin()
	// Используем QuoteMeta, так как DDL содержит спецсимволы
	mock.ExpectExec(regexp.QuoteMeta(initDDL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	conn, err := CreateConnection(context.Background(), "sqlmock", dsn)

	require.NoError(t, err)
	require.NotNil(t, conn)

	conn.Close()

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDBStorage_Ping verifies the Ping method.
func TestDBStorage_Ping(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer sqlDB.Close()

	mock.ExpectPing()

	storage := &DBStorage{DB: sqlDB}
	require.NoError(t, storage.Ping(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDBStorage_ImportLogs verifies fetching metrics from the DB.
func TestDBStorage_ImportLogs(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	storage := &DBStorage{DB: sqlDB}

	// Mocking rows
	rows := sqlmock.NewRows([]string{"ID", "MType", "Delta", "Value"}).
		AddRow("test_gauge", "gauge", nil, 123.456).
		AddRow("test_counter", "counter", 10, nil)

	mock.ExpectQuery("SELECT ID, MType, Delta, Value from metrics").
		WillReturnRows(rows)

	metrics, err := storage.ImportLogs(context.Background())
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	// Verify Gauge
	require.Equal(t, "test_gauge", metrics[0].ID)
	require.Equal(t, "gauge", metrics[0].MType)
	require.Nil(t, metrics[0].Delta)
	require.NotNil(t, metrics[0].Value)
	require.Equal(t, 123.456, *metrics[0].Value)

	// Verify Counter
	require.Equal(t, "test_counter", metrics[1].ID)
	require.Equal(t, "counter", metrics[1].MType)
	require.NotNil(t, metrics[1].Delta)
	require.Equal(t, int64(10), *metrics[1].Delta)
	require.Nil(t, metrics[1].Value)

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestDBStorage_FormattingLogs verifies the bulk insert/update transaction logic.
func TestDBStorage_FormattingLogs(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	storage := &DBStorage{DB: sqlDB}

	gauges := map[string]float64{"g1": 1.1}
	counters := map[string]int{"c1": 100}

	mock.ExpectBegin()

	// Ожидаем подготовку выражения для gauge
	mock.ExpectPrepare("INSERT INTO metrics .* 'gauge'")
	// Ожидаем подготовку выражения для counter
	mock.ExpectPrepare("INSERT INTO metrics .* 'counter'")

	// Ожидаем выполнение (порядок важен, в коде gauge идет первым)
	mock.ExpectExec("INSERT INTO metrics").
		WithArgs("g1", 1.1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO metrics").
		WithArgs("c1", int64(100)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Ожидаем закрытие prepared statements (defer Close())
	// sqlmock может требовать явного ожидания закрытия, если strict mode.
	// Обычно это не обязательно, но полезно знать.
	// Тут мы просто проверяем Commit.

	mock.ExpectCommit()

	err = storage.FormattingLogs(context.Background(), gauges, counters)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ExampleCreateConnection demonstrates how to initialize the DB storage.
// Note: This example uses a hypothetical "postgres" driver and connection string.
func ExampleCreateConnection() {
	// In a real application, replace "postgres" with your driver name
	// and use a valid DSN.
	// conn, err := CreateConnection(context.Background(), "postgres", "postgres://user:pass@localhost:5432/db")
	// if err != nil {
	//     log.Fatal(err)
	// }
	// defer conn.Close()

	fmt.Println("DB Connection initialized (example)")

	// Output:
	// DB Connection initialized (example)
}
