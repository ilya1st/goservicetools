/*
Here would be main program file located to work
*/
package main

import (
	"basecms/libraries/env"
	"fmt"
	"os"
)

// CustomAppStart app start there
type CustomAppStart struct {
	env.DefaultAppStartSetup
}

func main() {
	exitCode, err := env.AppStart(&CustomAppStart{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while starting app\n%v\n", err)
		os.Exit(exitCode)
	}
	env.AppRun()
}
