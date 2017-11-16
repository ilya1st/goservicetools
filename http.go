package goservicetools

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/ilya1st/configuration-go"
	"github.com/rs/zerolog"
)

/*
This file will contains functions we need fot init HTTP, HTTP/2, HTTPS server
First we would init just an http
*/

var (
	// todo: Socket here
	httpListener         net.Listener
	httpServer           *http.Server
	httpServerServeError error
	httpServerMutex      sync.RWMutex
	httpShutdownTimeout  int
	httpSsl              bool
	httpSslCert          string
	httpSslKey           string
)

//PrepareHTTPListener prepare http socket to run
// Notice: here we assume config is clean and normal
func PrepareHTTPListener(graceful bool, httpConfig configuration.IConfig) error {
	httpServerMutex.Lock()
	defer httpServerMutex.Unlock()
	l := GetSystemLogger()
	if httpConfig == nil || reflect.ValueOf(httpConfig).IsNil() {
		panic(fmt.Errorf("PrepareHTTPListener: httpconfig is nil"))
	}
	if graceful && (os.Getenv("GRACEFUL_HTTP_FD") != "") { // fd 0 stdin, 1 stdout, 2 stderr, 3 http
		fd, err := strconv.ParseInt(os.Getenv("GRACEFUL_HTTP_FD"), 10, 32)
		if err != nil {
			err = fmt.Errorf("PrepareHTTPListener: No variable GRACEFUL_HTTP_FD set for graceful start. Internal error: %v", err)
			if l == nil {
				panic(err)
			} else {
				l.Panic().Msg(err.Error())
			}
		}
		file := os.NewFile(uintptr(fd), "[httpsocket]")
		if file == nil {
			return fmt.Errorf("PrepareHTTPSocket: Not valid http listener file descriptor while graceful restart")
		}
		li, err := net.FileListener(file)
		if nil != err {
			err = fmt.Errorf("PrepareHTTPSocket: cannot prepare filelistener for http server. Error: %v", err)
			if l == nil {
				fmt.Fprint(os.Stderr, err.Error())
			} else {
				l.Error().Msg(err.Error())
			}
			return err
		}
		httpListener = li
		return nil
	}
	address, err := httpConfig.GetStringValue("address")
	if err != nil {
		panic(fmt.Errorf("PrepareHTTPSocket: config error: %v", err))
	}
	socketType, err := httpConfig.GetStringValue("socket_type")
	if err != nil {
		panic(fmt.Errorf("PrepareHTTPSocket: config error: %v", err))
	}
	switch socketType {
	case "unix":
	case "tcp":
	default:
		panic("PrepareHTTPSocket: wrong socket type")
	}
	httpListener, err = net.Listen(socketType, address)
	if err != nil {
		panic(fmt.Errorf("Error while listen socket: %v", err))
	}
	return nil
}

// GetHTTPListener returns internal http socket
func GetHTTPListener() net.Listener {
	httpServerMutex.RLock()
	defer httpServerMutex.RUnlock()
	return httpListener
}

// DropHTTPListener close socket. Call when not graceful there
func DropHTTPListener() {
	httpServerMutex.Lock()
	defer httpServerMutex.Unlock()
	if nil != httpListener {
		httpListener.Close()
		httpListener = nil
	}
}

// GetHTTPServer gets http server instance if started
func GetHTTPServer() *http.Server {
	httpServerMutex.RLock()
	defer httpServerMutex.RUnlock()
	return httpServer
}

type httpErrorWriter struct{ log *zerolog.Logger }

func (l *httpErrorWriter) Write(p []byte) (n int, err error) {
	if l.log == nil {
		return len(p), nil
	}
	l.log.Warn().Msg(string(p))
	return len(p), nil
}

// SetupHTTPServer setups http server(not stats!). for case of graceful gives their socket
// graceful or not here depends on was changed configuration file or not
// this one you must use after PrepareHTTPListener runned
func SetupHTTPServer(httpConfig configuration.IConfig) error {
	httpServerMutex.Lock()
	defer httpServerMutex.Unlock()
	l := GetSystemLogger()
	if l == nil || reflect.ValueOf(l).IsNil() {
		panic(fmt.Errorf("PrepareHTTPSocket: nil logger there"))
	}
	if httpConfig == nil || reflect.ValueOf(httpConfig).IsNil() {
		l.Fatal().Msg("SetupHTTPServer: httpconfig is nil. Will panic")
	}
	if httpServer != nil { // all already done
		return nil
	}
	if httpListener == nil {
		panic(fmt.Errorf("env.SetupHTTPServer: First setup httpListener - use PrepareHTTPListener() first"))
	}
	var err error

	httpShutdownTimeout, err = httpConfig.GetIntValue("shutdown_timeout")
	if err != nil {
		err = fmt.Errorf("PrepareHTTPListener: httpconfig shutdown_timeout error:%v. Will panic", err)
		l.Info().Msg(err.Error())
		panic(err)
	}
	if httpShutdownTimeout < 0 {
		err = fmt.Errorf("PrepareHTTPListener: shutdown_timeout is negative: no way to work. Check config")
		l.Info().Msg(err.Error())
		panic(err)
	}
	address, err := httpConfig.GetStringValue("address")
	if err != nil {
		panic(fmt.Errorf("PrepareHTTPSocket: config error: %v", err))
	}
	// no checks cause all is in CheckHTTPconfig
	sslConf, err := httpConfig.GetSubconfig("ssl")
	if err != nil {
		panic(fmt.Errorf("PrepareHTTPSocket: config error: %v", err))
	}
	httpSsl, err = sslConf.GetBooleanValue("ssl")
	if err != nil {
		panic(fmt.Errorf("PrepareHTTPSocket: config error: %v", err))
	}
	if httpSsl {
		httpSslCert, err = sslConf.GetStringValue("cert")
		if err != nil {
			panic(fmt.Errorf("PrepareHTTPSocket: config error: %v", err))
		}
		httpSslKey, err = sslConf.GetStringValue("key")
		if err != nil {
			panic(fmt.Errorf("PrepareHTTPSocket: config error: %v", err))
		}
	}
	httpServer = &http.Server{
		Addr: address,
		// setup our error log here
		ErrorLog: log.New(&httpErrorWriter{log: l}, "", 0),
	}
	return nil
}

// DropHTTPServer shut downs and drop server - not listener
func DropHTTPServer() {
	httpServerMutex.Lock()
	defer httpServerMutex.Unlock()
	if httpServer == nil {
		return
	}
	// TODO: add shutdown timeout to server config
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(httpShutdownTimeout)*time.Millisecond)
	defer func() {
		l := GetSystemLogger()
		if l != nil {
			l.Warn().Msgf("HTTP server shutdown hangs more then timeout %d", httpShutdownTimeout)
		}
		cancel()
	}()
	httpServer.Shutdown(ctx)
	httpServer = nil
}

// SetHTTPServeMux sets up server mux
func SetHTTPServeMux(mux http.Handler) {
	httpServerMutex.Lock()
	defer httpServerMutex.Unlock()
	if httpServer == nil {
		panic(fmt.Errorf("Cannot setup server mux to nil"))
	}
	httpServer.Handler = mux
}

// StartHTTPServer starts listen http with g
func StartHTTPServer() {
	httpServerMutex.Lock()
	defer httpServerMutex.Unlock()
	if httpServer == nil {
		panic(fmt.Errorf("Server is not. First init them"))
	}
	if httpListener == nil {
		panic(fmt.Errorf("HTTP Listener not prepared. Press prepare them"))
	}
	httpServerServeError = nil
	go func() { // cause Serve() is locking there - but we do not need that shit
		httpListener := GetHTTPListener()
		if httpListener == nil {
			return
		}
		var err error
		if httpSsl {
			err = httpServer.ServeTLS(httpListener, httpSslCert, httpSslKey)
		} else {
			err = httpServer.Serve(httpListener)
		}

		if err != http.ErrServerClosed {
			l := GetHTTPLogger()
			if l != nil {
				l.Error().Msgf("http server serve() error: %v", err)
			}
		}
	}()
}

func init() {
	httpServerMutex.Lock()
	defer httpServerMutex.Unlock()
	httpListener = nil
	httpServer = nil
	// default value
	httpShutdownTimeout = 10000
	httpServerServeError = nil
	httpSsl = false
	httpSslCert = ""
	httpSslKey = ""
}
