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
	"net/http"
	"os"
	"os/exec"
	"strconv"

	configuration "github.com/ilya1st/configuration-go"
	"github.com/ilya1st/goservicetools"
)

// CustomAppStart app start there
// Look at goservicetools.DefaultAppStartSetup
// understand on how to write applications
type CustomAppStart struct {
	goservicetools.DefaultAppStartSetup
	helloPort int
	config    configuration.IConfig
}

// newCustromAppStart create default object version of app
func newCustromAppStart() *CustomAppStart {
	return &CustomAppStart{helloPort: -1, config: nil}
}

// NeedHTTP implements IAppStartSetup.NeedHTTP() method
func (*CustomAppStart) NeedHTTP() bool {
	return true
}

// CommandLineHook implements IAppStartSetup.CommandLineHook() method
func (*CustomAppStart) CommandLineHook(cmdFlags map[string]string) {
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

// getHelloPort check cmdFlags value and internal config
// this is example on how to use internal app config
func (app *CustomAppStart) getHelloPort() int {
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
	panic("Error: no helloport defined in config file or arguments")
}

// CheckUserConfig checks user config parts
func (app *CustomAppStart) CheckUserConfig(mainconf configuration.IConfig) error {
	if app.config != nil {
		return nil
	}
	return fmt.Errorf("CheckUserConfig(): not implemented now")
}

// SystemSetup implements IAppStartSetup.SystemSetup() method
func (*CustomAppStart) SystemSetup(graceful bool) error {
	l := goservicetools.GetSystemLogger()
	if l != nil {
		l.Info().Msg("Running default system startup")
	}
	return nil
}

// HandleSignal handles signal from OS
func (*CustomAppStart) HandleSignal(sg os.Signal) error {
	// DO NOTHING CAUSE DO NOT NEED HANDLE ANYTHIG
	return nil
}

// ConfigureHTTPServer implements IAppStartSetup.ConfigureHTTPServer() method
func (*CustomAppStart) ConfigureHTTPServer(graceful bool) error {
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

// SystemStart start custom services
func (*CustomAppStart) SystemStart(graceful bool) error {
	return nil
}

// SystemShutdown implements IAppStartSetup.SystemShutdown
func (*CustomAppStart) SystemShutdown(graceful bool) error {
	return nil
}

// SetupOwnExtraFiles for graceful restart
func (*CustomAppStart) SetupOwnExtraFiles(cmd *exec.Cmd) error {
	/*
		place here something like that:
		cmd.ExtraFiles =
			files = append(cmd.ExtraFiles, <file from you listrner)
			cmd.Env = append(cmd.Env, fmt.Sprintf("GRACEFUL_YOUR_SERVICE_FD=%d", 2+len(cmd.ExtraFiles)))
	*/
	return nil
}

func main() {
	fmt.Println("Just open https://localhost:8000 when ready")
	exitCode, err := goservicetools.AppStart(newCustromAppStart())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while starting app\n%v\n", err)
		os.Exit(exitCode)
	}
	goservicetools.AppRun()
}
