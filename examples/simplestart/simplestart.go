/*
Here would be main program file located to work
To run:
ENV=dev go run simplestart.go
*/
package main

import (
	"fmt"
	"os"

	"github.com/ilya1st/goservicetools"
)

// CustomAppStart app start there
type CustomAppStart struct {
	goservicetools.DefaultAppStartSetup
}

func main() {
	fmt.Println("Just open https://localhost:8000 when ready")
	exitCode, err := goservicetools.AppStart(&CustomAppStart{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while starting app\n%v\n", err)
		os.Exit(exitCode)
	}
	goservicetools.AppRun()
}
