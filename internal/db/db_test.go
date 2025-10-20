package db

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCreateConnection(t *testing.T) {
	const dsn = "sqlmock_create_conn"

	sqlDB, mock, err := sqlmock.NewWithDSN(dsn, sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	mock.ExpectPing()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(initDDL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	conn, err := CreateConnection(context.Background(), "sqlmock", dsn)

	require.NoError(t, err)
	require.NotNil(t, conn)

	mock.ExpectClose()
	conn.Close()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBStoragePing(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	mock.ExpectPing()

	storage := &DBStorage{DB: sqlDB}
	require.NoError(t, storage.Ping(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}
