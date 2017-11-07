/*
Here would be main program file located to work
To run:
ENV=dev go run simplestart.go
Also it can do suid from lower ports
*/
package main

import (
	"fmt"
	"os"

	"github.com/ilya1st/goservicetools"
)

// CustomAppStart app start there
// Look at goservicetools.DefaultAppStartSetup
// understand on how to write applications
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
