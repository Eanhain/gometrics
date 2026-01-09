package configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVerPrint(t *testing.T) {
	// Можно временно подменить переменные, если они экспортируемые
	BuildVersion = "v1.0.0"
	BuildDate = "2023-01-01"
	BuildCommit = "abcdef"

	got := BuildVerPrint()
	assert.Contains(t, got, "Build version: v1.0.0")
	assert.Contains(t, got, "Build date: 2023-01-01")
	assert.Contains(t, got, "Build commit: abcdef")
}
