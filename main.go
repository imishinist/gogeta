package main

import (
	"fmt"
)

// Set at linking time
var (
	Commit  string
	Date    string
	Version string
)

func main() {
	fmt.Println("Gogeta Attack")
	fmt.Printf("Commit: %s, Date: %s\n", Commit, Date)
}
