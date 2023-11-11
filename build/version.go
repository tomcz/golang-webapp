package build

import "fmt"

var commit string

func Commit() string {
	return commit
}

func Version() string {
	buildName := "dev"
	if IsProd {
		buildName = "prod"
	}
	return fmt.Sprintf("%s-%s", commit, buildName)
}
