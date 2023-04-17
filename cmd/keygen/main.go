package main

import (
	"fmt"
	"log"

	"github.com/tomcz/golang-webapp/internal/webapp"
)

func main() {
	key, err := webapp.RandomKey()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(key)
}
