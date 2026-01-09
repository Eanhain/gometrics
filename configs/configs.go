package configs

import (
	"fmt"
)

var (
	BuildVersion string = "N/A"
	BuildDate    string = "N/A"
	BuildCommit  string = "N/A"
)

// Example boot:
// go run -ldflags "-X gometrics/configs.BuildVersion=v1.0.1 -X 'gometrics/configs.BuildDate=$(date +'%Y/%m/%d %H:%M:%S')' -X 'gometrics/configs.BuildCommit=34sd'" main.go
func BuildVerPrint() string {
	return fmt.Sprintf("Build version: %s\nBuild date: %s\nBuild commit: %s", BuildVersion, BuildDate, BuildCommit)
}
