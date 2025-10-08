package db

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCreateConnection(t *testing.T) {
	const dsn = "sqlmock_create_conn"

	sqlDB, mock, err := sqlmock.NewWithDSN(dsn)
	require.NoError(t, err)
	mock.ExpectClose()
	require.NoError(t, sqlDB.Close())

	conn, err := CreateConnection("sqlmock", dsn)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMemStoragePingDB(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer sqlDB.Close()

	mock.ExpectPing()

	storage := &MemStorage{DB: sqlDB}
	require.NoError(t, storage.PingDB(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}
