package db

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

type MemStorage struct {
	*sql.DB
}

func CreateConnection(dbType, connectionString string) (*MemStorage, error) {
	db, err := sql.Open(dbType, connectionString)
	if err != nil {
		log.Fatal(err)
	}
	if err != nil {
		return &MemStorage{}, err
	}
	return &MemStorage{db}, nil
}

func (s *MemStorage) PingDB(ctx context.Context) error {
	return s.PingContext(ctx)
}
