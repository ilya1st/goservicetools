# goservicetools - library to write services on go

Framework to make service from your application(with different log types, http, suid, graceful restart)

## Overview

This pacakage was written for case you need write own unix service in golang.
For example. I want my application to have following things:

 1. Application must have own configuration file and tools to read them. And must work at least in 3 enviromnents defined by ENV variable or command line option(prod, dev, test)
 1. Application must be able write own log files and support at least 2 formats(plain and json) an 4 outputs(stderr, file, syslog, and no outout) with syslog like facilities.
 1. Application must be able work as daemon:
    1. Support SIGINT, SIGTERM
    1. Support work with logrotate and reopen logs on SIGHUP signal.
    1. Application must be able open HTTP and HTTPS service and give user configure them
    1. Application must have support for setuid(+self restart) to support open lower ports without using docker or capabilities
    1. Application must support graceful self restart on SIGUSR1 signal with no reopening ports(it must transmit port listener descriptors to new instance)
    1. Things like logfile, pidfile

## Quickstart.

To setup your application you must define child struct from IAppStartSetup interface or from DefaultAppStartSetup. On code documentation to IAppStartSetup it's described call order of methods.

So this is your first program code.

```go
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
```
Library contains additional API to work with loggers, configuration files, command line arguments.
There is examples directory you can see on how to work with library components

## External libraries used

* github.com/rs/zerolog for logging
* github.com/ilya1st/rotatewriter to support log rotate on SIGHUP, 
* github.com/ilya1st/configuration-go to support HJSON(json with not strict syntax) to work with configuration files

## Application configuration file

We use HJSON format. Configuration file contains 3 main sections:

* prod - for production environments
* dev - for development
* test - for test environment(e.g. run unit tests)

Each section contains own options.

You can see in conf/config.hjson documented configuration file with different options.
You can add own options there, e.g. number of ports user want to listen. See helloservice example on how to work with configuration.

## Graceful port reopening

See AppStop() internals to understand on how does that works and helloservice example where gracefully  restarted does not need open socket to listen - it gives file descriptor from previous instance.
