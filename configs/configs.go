package configs

import (
	"fmt"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

// Example boot:
// go run -ldflags "-X gometrics/configs.buildVersion=v1.0.1 -X 'gometrics/configs.buildDate=$(date +'%Y/%m/%d %H:%M:%S')' -X 'gometrics/configs.buildCommit=34sd'" main.go
func BuildVerPrint() string {
	return fmt.Sprintf("Build version: %s\nBuild date: %s\nBuild commit: %s", buildVersion, buildDate, buildCommit)
}
