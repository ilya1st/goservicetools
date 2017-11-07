/*
Here would be main program file located to work
To run:
ENV=dev go run helloservice.go
Also it can do suid from lower ports
We listen on default HTTP port and answer hello there and open specific
config with special section for our miniservice
*/
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"

	configuration "github.com/ilya1st/configuration-go"
	"github.com/ilya1st/goservicetools"
)

// helloApp app start there
// Look at goservicetools.DefaultAppStartSetup
// understand on how to write applications
type helloApp struct {
	goservicetools.DefaultAppStartSetup
	helloPort int
	config    configuration.IConfig
	listener  net.Listener
	rMutex    sync.RWMutex
}

// newHelloAppStart create default object version of app
func newHelloApp() *helloApp {
	return &helloApp{helloPort: -1, config: nil, listener: nil}
}

// NeedHTTP implements IAppStartSetup.NeedHTTP() method
func (*helloApp) NeedHTTP() bool {
	return true
}

// CommandLineHook implements IAppStartSetup.CommandLineHook() method
func (app *helloApp) CommandLineHook(cmdFlags map[string]string) {
	// redefine here hello port argument
	var helloport = ""
	flag.StringVar(&helloport, "helloport", "", "override configuration hello port")
	flag.Parse()
	if "" == helloport { // no add to map
		return
	}
	_, err := strconv.ParseUint(helloport, 10, 16)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing helloport parameter - must be port number: %v", err)
		goservicetools.Exit(goservicetools.ExitCodeConfigError)
	}
	cmdFlags["helloport"] = helloport
}

// CheckUserConfig checks user config parts
func (app *helloApp) CheckUserConfig(mainconf configuration.IConfig) error {
	app.rMutex.Lock()
	defer app.rMutex.Unlock()
	port, err := mainconf.GetIntValue("hello", "port")
	if err != nil {
		return fmt.Errorf("Parsing hello service section of configuration error %v", err)
	}
	if port < 0 {
		return fmt.Errorf("Port value can not be negative")
	}
	return nil
}

// SystemSetup implements IAppStartSetup.SystemSetup() method
// here we open socket, setup listener(from graceful etc)
func (app *helloApp) SystemSetup(graceful bool) error {
	conf, err := configuration.GetConfigInstance("main")
	if err != nil {
		panic(fmt.Errorf("getHelloPort() config error %v", err))
	}
	_env, err := goservicetools.GetEnvironment()
	if err != nil {
		panic(fmt.Errorf("getHelloPort()  error on getting current working environment: %v", err))
	}
	config, err := conf.GetSubconfig(_env)
	goservicetools.GetSystemLogger().Info().Msg("Prepared working application config")
	app.config = config
	goservicetools.GetSystemLogger().Info().Msg("Setting up hello app")
	helloPort := app.getHelloPort()
	if helloPort < 0 {
		return fmt.Errorf("There is negative hello port in app")
	}
	goservicetools.GetSystemLogger().Info().Msgf("Found configured port value %d", helloPort)
	if graceful && (os.Getenv("GRACEFUL_HELLO_FD") != "") {
		fd, err := strconv.ParseInt(os.Getenv("GRACEFUL_HTTP_FD"), 10, 32)
		if err != nil {
			err = fmt.Errorf("No variable GRACEFUL_HTTP_FD set for graceful start. Internal error: %v", err)
			goservicetools.GetSystemLogger().Panic().Msg(err.Error())
		}
		file := os.NewFile(uintptr(fd), "[hellosocket]")
		if file == nil {
			return fmt.Errorf("PrepareHTTPSocket: Not valid hello listener file descriptor while graceful restart")
		}
		li, err := net.FileListener(file)
		if nil != err {
			err = fmt.Errorf("SystemSetup: cannot prepare filelistener for hello server. Error: %v", err)
			goservicetools.GetSystemLogger().Error().Msg(err.Error())
			return err
		}
		app.listener = li
		return nil
	}
	address := fmt.Sprintf(":%d", helloPort)
	fmt.Println(address)
	goservicetools.GetSystemLogger().Info().Msgf("Try setup hello listener on address: %v", address)
	httpListener, err := net.Listen("tcp", address)
	if err != nil {
		goservicetools.GetSystemLogger().Fatal().Msgf("Error while listen socket: %v", err)
	}
	// in system start we would sy hello on each connect
	app.listener = httpListener
	return nil
}

// getHelloPort check cmdFlags value and internal config
// this is example on how to use internal app config
func (app *helloApp) getHelloPort() int {
	app.rMutex.Lock()
	defer app.rMutex.Unlock()
	if app.helloPort >= 0 {
		return app.helloPort
	}
	// when init done it returns ready map, not runs twice
	cmdFlags := goservicetools.GetCommandLineFlags(nil)
	h, ok := cmdFlags["helloport"]
	if ok {
		tmp, err := strconv.ParseUint(h, 10, 16)
		if err != nil { // cause already checked
			panic(fmt.Errorf("error while checking helloport value from commandline flags: %v", err))
		}
		app.helloPort = int(tmp)
		return app.helloPort
	}
	conf, err := configuration.GetConfigInstance("main")
	if err != nil {
		panic(fmt.Errorf("getHelloPort() config error %v", err))
	}
	_env, err := goservicetools.GetEnvironment()
	if err != nil {
		panic(fmt.Errorf("getHelloPort()  error on getting current working environment: %v", err))
	}
	config, err := conf.GetSubconfig(_env)
	if err != nil {
		panic(fmt.Errorf("getHelloPort()  error on getting configuration for concurrent working environment: %v", err))
	}

	helloPort, err := config.GetIntValue("hello", "port")
	if err != nil {
		panic(fmt.Errorf("Error: no hello/port defined in config file or arguments: %v", err))
	}
	app.helloPort = helloPort
	return app.helloPort
}

// HandleSignal handles signal from OS
func (*helloApp) HandleSignal(sg os.Signal) error {
	// DO NOTHING CAUSE DO NOT NEED HANDLE ANYTHING
	goservicetools.GetSystemLogger().Info().Msgf("Caught signal: %v", sg)
	return nil
}

// ConfigureHTTPServer implements IAppStartSetup.ConfigureHTTPServer() method
func (*helloApp) ConfigureHTTPServer(graceful bool) error {
	newMux := http.NewServeMux()
	newMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Hello!")
		// Get X-Forwarded-For from header
		xfwdfor := r.Header.Get("X-Forwarded-for")
		xrealip := r.Header.Get("X-Real-IP")
		e := goservicetools.GetHTTPLogger().Info().
			Str("ip", r.RemoteAddr)
		if xfwdfor != "" {
			e = e.Str("x-forwarded-for", xfwdfor)
		}
		if xrealip != "" {
			e = e.Str("x-real-ip", xrealip)
		}
		e = e.
			Str("method", r.Method).
			Str("host", r.Host).
			Str("url", r.URL.Path).
			Str("agent", r.UserAgent()).
			Str("referer", r.Referer()).
			Int("code", http.StatusOK)
		e.
			Msg("request")
	})
	// TODO: move new mux parameter to appstart
	goservicetools.SetHTTPServeMux(newMux)
	l := goservicetools.GetSystemLogger()
	if l != nil {
		l.Info().Msg("Default http server set up")
	}
	return nil
}

// SystemStart start custom services with prepared listeners in SystemSetup
func (*helloApp) SystemStart(graceful bool) error {

	return nil
}

// SystemShutdown implements IAppStartSetup.SystemShutdown
func (app *helloApp) SystemShutdown(graceful bool) error {
	if !graceful {
		app.listener.Close()
	}
	return nil
}

// SetupOwnExtraFiles for graceful restart
// and setup some environment variables for them
func (app *helloApp) SetupOwnExtraFiles(cmd *exec.Cmd, newConfig configuration.IConfig) error {
	/*
		place here something like that:
		cmd.ExtraFiles =
			files = append(cmd.ExtraFiles, <file from you listrner)
			cmd.Env = append(cmd.Env, fmt.Sprintf("GRACEFUL_YOUR_SERVICE_FD=%d", 2+len(cmd.ExtraFiles)))
	*/
	if app.listener == nil {
		goservicetools.GetSystemLogger().Fatal().Msg("No listener in helloApp was perepared")
	}
	_env, err := goservicetools.GetEnvironment()
	if err != nil {
		goservicetools.GetSystemLogger().Fatal().Msgf("SetupOwnExtraFiles: error while getting current envirnment: %v", err)
	}
	newPort, err := newConfig.GetIntValue(_env, "hello", "port")
	if err != nil {
		goservicetools.GetSystemLogger().Fatal().Msgf("SetupOwnExtraFiles: error while new port value from new config: %v", err)
	}
	if newPort != app.helloPort {
		app.listener.Close()
	}
	// Warning: here we assume config not changes
	return nil
}

func main() {
	fmt.Println("Just open https://localhost:8000 when ready")
	exitCode, err := goservicetools.AppStart(newHelloApp())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while starting app\n%v\n", err)
		os.Exit(exitCode)
	}
	goservicetools.AppRun()
}
