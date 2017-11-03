package env

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	configuration "github.com/ilya1st/configuration-go"
)

// Here we will place more complex functions about application start, log setup sockets, etc

// AppStartSetup structure with setup application hooks
// goes to
// SystemSetup starts database connections and same other things

// IAppStartSetup appstart setup for application
type IAppStartSetup interface {
	// NeedHTTP does app need http sevice or not
	NeedHTTP() bool
	// CommandLineHookadds additional command line flags to global cmdFlags structure
	CommandLineHook(cmdFlags map[string]string)
	// checks user config parts
	CheckUserConfig(mainconf configuration.IConfig) error
	// SystemSetup setup other suid ports here
	SystemSetup(graceful bool) error
	// HandleSignal handles signal from OS
	HandleSignal(sg os.Signal) error
	// Set up custom http mux, log, etc
	ConfigureHTTPServer(graceful bool) error
	// Start custom services - after ports are ready
	SystemStart(graceful bool) error
	// Run on app shutdown
	SystemShutdown(graceful bool) error
	// SetupOwnExtraFiles for graceful restart - transfer suid ports to graceful child
	SetupOwnExtraFiles(cmd *exec.Cmd) error
}

// DefaultAppStartSetup implements IAppStartSetup
// this is default app start setup and an example on how to write own appstart class
type DefaultAppStartSetup struct {
}

// NeedHTTP implements IAppStartSetup.NeedHTTP() method
func (*DefaultAppStartSetup) NeedHTTP() bool {
	return true
}

// CommandLineHook implements IAppStartSetup.CommandLineHook() method
func (*DefaultAppStartSetup) CommandLineHook(cmdFlags map[string]string) {
	// do nothing
}

// CheckUserConfig checks user config parts
func (*DefaultAppStartSetup) CheckUserConfig(mainconf configuration.IConfig) error {
	return nil
}

// SystemSetup implements IAppStartSetup.SystemSetup() method
func (*DefaultAppStartSetup) SystemSetup(graceful bool) error {
	l := GetSystemLogger()
	if l != nil {
		l.Info().Msg("Running default system startup")
	}
	return nil
}

// HandleSignal handles signal from OS
func (*DefaultAppStartSetup) HandleSignal(sg os.Signal) error {
	// DO NOTHING CAUSE DO NOT NEED HANDLE ANYTHIG
	return nil
}

// ConfigureHTTPServer implements IAppStartSetup.ConfigureHTTPServer() method
func (*DefaultAppStartSetup) ConfigureHTTPServer(graceful bool) error {
	newMux := http.NewServeMux()
	newMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// TODO: http log sample there
		fmt.Fprintf(w, "This is default server mux. See defaultAppStartSetup setting IAppStartSetup in appstart.go file. You can create you one. URI: %v", r.URL.Path)
	})
	// TODO: move new mux parameter to appstart
	SetHTTPServeMux(newMux)
	l := GetSystemLogger()
	if l != nil {
		l.Info().Msg("Default http server set up")
	}
	return nil
}

// SystemStart start custom services
func (*DefaultAppStartSetup) SystemStart(graceful bool) error {
	return nil
}

// SystemShutdown implements IAppStartSetup.SystemShutdown
func (*DefaultAppStartSetup) SystemShutdown(graceful bool) error {
	return nil
}

// SetupOwnExtraFiles for graceful restart
func (*DefaultAppStartSetup) SetupOwnExtraFiles(cmd *exec.Cmd) error {
	/*
		place here something like that:
		cmd.ExtraFiles =
			files = append(cmd.ExtraFiles, <file from you listrner)
			cmd.Env = append(cmd.Env, fmt.Sprintf("GRACEFUL_YOUR_SERVICE_FD=%d", 2+len(cmd.ExtraFiles)))
	*/
	return nil
}

var (
	appAppStartSetup IAppStartSetup
	appConfigPath    string
)

// AppStart app start function
// if graceful - then to app transmited http socket in fd 3 https in fd 4 etc.
func AppStart(setup IAppStartSetup) (exitCode int, err error) {
	// setuid will be also graceful
	graceful := os.Getenv("GRACEFUL_START") == "YES"
	if setup == nil {
		appAppStartSetup = &DefaultAppStartSetup{}
	} else {
		appAppStartSetup = setup
	}
	// main environment init
	cmdp := GetCommandLineFlags(appAppStartSetup.CommandLineHook)
	_env, ok := cmdp["env"]
	if ok {
		err = ValidateEnv(_env)
		if err != nil {
			return ExitCodeWrongEnv, fmt.Errorf("%s", err)
		}
		// init internal environment value
		_env, err = GetEnvironment(true, _env)
	} else {
		if os.Getenv("ENV") == "" {
			// by default we think we work with production environment
			_env, err = GetEnvironment(true, "prod")
		} else {
			_env, err = GetEnvironment()
		}
	}
	if nil != err {
		return ExitCodeWrongEnv, fmt.Errorf("Environment init error %v", err)
	}
	// now try load configuration to memory
	_config, ok := cmdp["config"]
	if !ok {
		return ExitCodeConfigError, fmt.Errorf("There is no configuration file in commandline arguments")
	}
	appConfigPath = _config

	// here goes app startup at all. TODO: think AppStartup and AppDown functions
	_, err = configuration.GetConfigInstance("main", "HJSON", _config)
	if err != nil {
		return ExitCodeConfigError, fmt.Errorf("Error occured while loading configuration.\nConfig file: %s\nError: %s\nExiting", _config, err)
	}
	conf, err := configuration.GetConfigInstance("main")
	if err != nil {
		return ExitCodeConfigError, err
	}
	_env, err = GetEnvironment()
	if err != nil {
		return ExitCodeConfigError, err
	}
	err = CheckAppConfig(conf)
	if err != nil {
		return ExitCodeConfigError, fmt.Errorf("Configuration file error %v", err)
	}
	workdir, _ := conf.GetStringValue(_env, "workdir")
	if workdir != "" {
		st, err := os.Stat(workdir)
		if err != nil {
			return ExitCodeConfigError, fmt.Errorf("Working directory stat error: %v", err)
		}
		if !st.IsDir() {
			return ExitCodeConfigError, fmt.Errorf("Working directory workdir %s is not a directury", workdir)
		}
		err = os.Chdir(workdir)
		if err != nil {
			return ExitCodeConfigError, fmt.Errorf("Cannot chdir to Working directory workdir %s, error: %v", workdir, err)
		}
	}
	setuidConf, err := conf.GetSubconfig(_env, "setuid")
	if err != nil {
		switch err.(type) {
		case *configuration.ConfigItemNotFound:
			err = nil
		default:
		}
	}
	if err != nil {
		return ExitCodeConfigError, fmt.Errorf("Application configuration error: %v", err)
	}
	if setuidConf != nil {
		setuid, err := setuidConf.GetBooleanValue("setuid")
		if err != nil {
			return ExitCodeConfigError, fmt.Errorf("Application configuration error: %v", err)
		}
		if setuid && !graceful { // run all things and graceful stop
			err = appAppStartSetup.SystemSetup(false)
			if err != nil {
				return ExitUserDefinedCodeError, fmt.Errorf(`Error occured while setting up HTTP Listener. Error: %v\nExiting`, err)
			}
			if appAppStartSetup.NeedHTTP() {
				httpConf, _ := conf.GetSubconfig(_env, "http")
				err = PrepareHTTPListener(false, httpConf)
			}
			if err != nil {
				return ExitHTTPStartError, fmt.Errorf(`Error occured while setting up HTTP Listener. Error: %v\nExiting`, err)
			}
			sd, err := GetSetUIDGIDData(setuidConf)
			if err != nil {
				return ExitCodeConfigError, fmt.Errorf("Got getting uid and gid error: %v", err)
			}
			return AppStop(true, sd)
		}
	}
	h, _ := conf.GetSubconfig(_env, "lockfile") // no err check above cause of we use err = CheckAppConfig(conf)
	err = SetupLockFile(h)
	if err != nil {
		return ExitCodeLockfileError, err
	}
	p, _ := conf.GetSubconfig(_env, "pidfile") // no err check above cause of we use err = CheckAppConfig(conf)
	err = SetupPidfile(p)
	if err != nil {
		return ExitCodeLockfileError, err
	}
	SetupSighupHandlers()
	systemLogConf, _ := conf.GetSubconfig(_env, "logs", "system") // no err check above cause of we use err = CheckAppConfig(conf)
	_, err = SetupLog("system", systemLogConf)
	if err != nil {
		return ExitCodeConfigError, fmt.Errorf(`Error occured while loading configuration.
			Cannot setup system log file.
			Error: %v\nExiting`, err)
	}
	GetSystemLogger().Info().Msg("Application starts. System log ready")
	SetupSighupRotationForLogs()
	err = appAppStartSetup.SystemSetup(graceful)

	if err != nil {
		return ExitUserDefinedCodeError, fmt.Errorf(`Error make system inititalization %v`, err)
	}

	GetSystemLogger().Info().Msg("Setting up http logs done")
	GetSystemLogger().Info().Msg("Setting up http server. Prepare http listener")
	// yes it must be there withous errors after config check
	if appAppStartSetup.NeedHTTP() {
		GetSystemLogger().Info().Msg("Setting up http logs")
		httpLogConf, _ := conf.GetSubconfig(_env, "logs", "http") // no err check above cause of we use err = CheckAppConfig(conf)
		_, err = SetupLog("http", httpLogConf)
		if err != nil {
			return ExitCodeConfigError, fmt.Errorf(`Error occured while loading configuration.
				Cannot setup http log file.
				Error: %v\nExiting`, err)
		}
		httpConf, _ := conf.GetSubconfig(_env, "http")
		err = PrepareHTTPListener(graceful, httpConf)
		if err != nil {
			return ExitHTTPStartError, fmt.Errorf(`Error occured while setting up HTTP Listener. Error: %v\nExiting`, err)
		}
		addr, _ := httpConf.GetStringValue("address")
		GetSystemLogger().Info().Msgf("Http listener ready. address: %v. Setting up HTTP server itself", addr)
		SetupHTTPServer(httpConf)
		err = appAppStartSetup.ConfigureHTTPServer(graceful)
		StartHTTPServer()
		GetSystemLogger().Info().Msg("HTTP server started")
	}
	// starting other than default HTTP custom services
	err = appAppStartSetup.SystemStart(graceful)
	if err != nil {
		return ExitCustomAppError, fmt.Errorf(`Error occured while starting custom services. Error: %v\nExiting`, err)
	}
	return 0, nil
}

var appStopMutex sync.Mutex

// AppStop is intended to control app stop while things:
// SIGINT handling, SIGTERM handling, SIGUSR handling for a graceful exit to give socket descriptors
// to new process
// if graceful==true then function tries make graceful restart
// function makes app restart and also makes it's suid start
func AppStop(graceful bool, sd *SetuidData) (exitCode int, err error) {
	appStopMutex.Lock()
	defer appStopMutex.Unlock()
	var (
		newConfig configuration.IConfig
	)
	newConfig = nil
	conf, err := configuration.GetConfigInstance("main")
	if err != nil {
		GetSystemLogger().Fatal().Msgf("Config is not set to run %v", err)
	}
	_env, err := GetEnvironment()
	if err != nil {
		GetSystemLogger().Fatal().Msgf("Environment is not set to run: %v", err)
	}
	if graceful {
		newConfig, err = configuration.GetConfigInstance(nil, "HJSON", appConfigPath)

		if err != nil {
			GetSystemLogger().Fatal().Msgf("Can not restart. Application config file was broken: %v", err)
		}
		err = CheckAppConfig(newConfig)
		if err != nil {
			GetSystemLogger().Fatal().Msgf("Can not restart. Application config file contains errors: %v", err)
		}
		// WHAT IS THE RIGHT WAy HERE?
	}
	_, err = configuration.GetConfigInstance("main")
	if err != nil {
		return ExitCodeConfigError, err
	}
	_, err = GetEnvironment()
	if err != nil {
		return ExitCodeWrongEnv, err
	}
	CleanupSighupHandlers()
	l := GetSystemLogger()
	if l != nil {
		l.Info().Msg("Shutting down http")
	}
	if appAppStartSetup.NeedHTTP() && !graceful {
		DropHTTPServer()
		if !graceful {
			DropHTTPListener()
		}
	}
	err = appAppStartSetup.SystemShutdown(graceful)
	if err != nil {
		if l == nil {
			panic(fmt.Errorf("Error during system shutdown occured: %v", err))
		}
		l.Fatal().Msgf("Error during system shutdown occured: %v", err)
	}
	DropLogger("http")
	if !graceful {
		DropLogger("system")
	}
	DropLockFile()
	DropPidfile()
	if graceful {
		cmd := exec.Command(os.Args[0], os.Args[1:]...)

		cmd.ExtraFiles = []*os.File{}
		cmd.Env = os.Environ()
		if appAppStartSetup.NeedHTTP() {

			oldAddr, err := conf.GetStringValue(_env, "http", "address")
			if err != nil {
				if l == nil {
					panic(fmt.Errorf("AppStop() restart: No address in the config"))
				}
				l.Fatal().Msg("AppStop() restart: No address in the config")
			}
			oldSocketType, err := conf.GetStringValue(_env, "http", "socket_type")
			if err != nil {
				if l == nil {
					panic(fmt.Errorf("AppStop() restart: No addsocket_type  in the config"))
				}
				l.Fatal().Msg("AppStop() restart: No addsocket_type  in the config")
			}

			newAddr, err := newConfig.GetStringValue(_env, "http", "address")
			if err != nil {
				if l == nil {
					panic(fmt.Errorf("AppStop() restart: No address in the new config"))
				}
				l.Fatal().Msg("AppStop() restart: No address in the new config")
			}
			newSocketType, err := newConfig.GetStringValue(_env, "http", "socket_type")
			if err != nil {
				if l == nil {
					panic(fmt.Errorf("AppStop() restart: No addsocket_type  in the new config"))
				}
				l.Fatal().Msg("AppStop() restart: No addsocket_type  in the new config")
			}

			if oldAddr == newAddr && oldSocketType == newSocketType {
				li := GetHTTPListener()
				if li == nil {
					if l == nil {
						panic(fmt.Errorf("HTTP listener nil"))
					}
					l.Fatal().Msgf("HTTP listener nil")
				}
				var f *os.File
				switch v := li.(type) {
				case *net.TCPListener:
					f, err = v.File()
				case *net.UnixListener:
					f, err = v.File()
				default:
					if l == nil {
						panic(fmt.Errorf("Wrong tcp or unix listener"))
					}
					l.Fatal().Msgf("Wrong tcp or unix listener")
				}
				if err != nil {
					return 0, err
				}

				cmd.ExtraFiles = append(cmd.ExtraFiles, f)
				cmd.Env = append(cmd.Env, fmt.Sprintf("GRACEFUL_HTTP_FD=%d", 2+len(cmd.ExtraFiles)))
			}
		}
		cmd.Env = append(cmd.Env, "GRACEFUL_START=YES")
		if sd != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setsid: true,
				Credential: &syscall.Credential{
					Uid: sd.uid,
					Gid: sd.gid,
				},
			}
		}
		err := appAppStartSetup.SetupOwnExtraFiles(cmd)
		if err != nil {
			if l == nil {
				panic(fmt.Errorf("custom appstart error: %v", err))
			}
			l.Fatal().Msgf("custom appstart error: %v", err)
		}
		if l != nil {
			l.Info().Msg("Graceful application restart")
		}
		DropLogger("system")
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		fmt.Printf("Spawned process %d, exiting\n", cmd.Process.Pid)
		cmd.Process.Release()
		os.Exit(0)
	}
	return 0, nil
}

var appRunChan chan os.Signal
var appRunMutex sync.Mutex

// AppRun just to run app when we all do
// here is no way cover with tests cause need manually test them or make intagration tests there
func AppRun() {
	appRunChan = make(chan os.Signal, 1)
	signal.Notify(appRunChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
goloop:
	for {
		sg, ok := <-appRunChan
		appRunMutex.Lock()
		if !ok {
			sighupMutex.Unlock()
			break goloop
		}
		switch sg {
		case syscall.SIGINT:
			fmt.Println("SIGInt")
			fallthrough
		case syscall.SIGTERM:
			appAppStartSetup.HandleSignal(sg)
			exitCode, err := AppStop(false, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while app shutdown occured: %v", err)
			}
			Exit(exitCode)
		case syscall.SIGUSR1:
			appAppStartSetup.HandleSignal(sg)
			exitCode, err := AppStop(true, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while app shutdown occured: %v", err)
				Exit(exitCode)
			}
			Exit(exitCode)
		default:
		}
		appRunMutex.Unlock()
	}
}

func init() {
	appAppStartSetup = nil
	appConfigPath = ""
}
