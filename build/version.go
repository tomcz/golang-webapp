package build

import "fmt"

var commit string

func Version() string {
	return fmt.Sprintf("%s-%s", commit, buildName)
}
