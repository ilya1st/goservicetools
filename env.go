package goservicetools

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"sync"
	"syscall"

	"github.com/theckman/go-flock"

	"github.com/ilya1st/rotatewriter"

	"github.com/ilya1st/configuration-go"

	"log/syslog"

	"github.com/rs/zerolog"
)

// variants of exit codes
const (
	ExitCodeNormalExit = iota
	ExitCodeWrongEnv
	ExitCodeConfigError
	ExitCodeLockfileError
	ExitHTTPStartError
	ExitUserDefinedCodeError
	ExitHTTPServeError
	ExitCustomAppError
	ExitSuidError
)

/*
This package intended to solve application environment isses,
Goal is store basic computed unix environment settings to play with them from other parts
of program
*/

// this is current environment
// dev - development
// test - test environment
// prod - production
var _Env string

// ValidateEnv checks correct _Env variable value
func ValidateEnv(str string) error {
	switch str {
	case "dev":
		fallthrough
	case "test":
		fallthrough
	case "prod":
		return nil
	default:
		return fmt.Errorf("Wrong ENV definition: %s, correct variants: dev, test, prod", str)
	}
}

// GetEnvironment returns configured application environment
// function will have 2 parameters
// first used to reset default false
// second used to reset to default value - used in error cases
// I need them for testing purposes too
func GetEnvironment(sl ...interface{}) (env string, err error) {
	resetEnv := false
	if len(sl) > 0 {
		switch v := sl[0].(type) {
		case bool:
			resetEnv = v
		default:
			return "", fmt.Errorf("First argument must be boolean")
		}
	}
	if !resetEnv && ("" != _Env) {
		return _Env, nil
	}
	// second parameter
	defEnv := ""
	if len(sl) > 1 {
		switch v := sl[1].(type) {
		case string:
			defEnv = v
		default:
			return "", fmt.Errorf("Second argument must be a string")
		}
	}
	if "" != defEnv {
		err := ValidateEnv(defEnv)
		if nil == err {
			_Env = defEnv
			return _Env, nil
		}
		return "", err
	}
	// now we get two parameters: from Environment and from CMD
	// and solve what to use
	// this is last chance way - do not think it's a good solution
	envEnv := os.Getenv("ENV")
	err = ValidateEnv(envEnv)
	if err != nil {
		return "", err
	}
	_Env = envEnv
	return _Env, nil
}

var _cmdFlags map[string]string

// GetCommandLineFlags generates parameter map from commanline and reports errors
// on usage
// CustomFlags function add to cmdFlags additional flags all the same as in function
func GetCommandLineFlags(CustomFlags func(cmdFlags map[string]string)) map[string]string {
	if nil != _cmdFlags {
		return _cmdFlags
	}
	_cmdFlags = map[string]string{}
	var env string
	flag.StringVar(&env, "env", "", `dev|test|prod environment types
		dev: development
		test: test
		prod: production
		On environment type depends what section of configuration file will be used.
		Also you can use ENV environment variable.
		Default is "prod". Program will work with production part of config.
`)
	var config string
	flag.StringVar(&config, "config", "./conf/config.hjson", "Path to configuration file to run")
	flag.Parse()
	if env != "" {
		_cmdFlags["env"] = env
	}
	if config != "" {
		_cmdFlags["config"] = config
	}
	if CustomFlags != nil {
		CustomFlags(_cmdFlags)
	}
	return _cmdFlags
}

// internal storage for handlers
var (
	sighupHandlers []func()
	sighupMutex    sync.Mutex
)

// AddSighupHandler adds sighup handler to catch suck things as log rotation
func AddSighupHandler(handler func()) {
	sighupMutex.Lock()
	defer sighupMutex.Unlock()
	if nil == sighupHandlers {
		sighupHandlers = make([]func(), 0, 10)
	}
	sighupHandlers = append(sighupHandlers, handler)
}

var (
	sighupListSet bool
	sighupChan    chan os.Signal
)

// SetupSighupHandlers sets handler at startup got SIGHUP kill
// We need that e.g. to rotate logs
func SetupSighupHandlers() {
	sighupMutex.Lock()
	defer sighupMutex.Unlock()
	if sighupListSet {
		return
	}
	sighupChan := make(chan os.Signal, 1)
	signal.Notify(sighupChan, syscall.SIGHUP)
	sighupListSet = true
	go func() {
	goloop:
		for {
			_, ok := <-sighupChan
			sighupMutex.Lock()
			if !ok {
				sighupMutex.Unlock()
				break goloop
			}
			if nil == sighupHandlers {
				sighupMutex.Unlock()
				continue
			}
			for _, f := range sighupHandlers {
				go f()
			}
			sighupMutex.Unlock()
		}
	}()
}

// CleanupSighupHandlers clears channel and routines
func CleanupSighupHandlers() {
	sighupMutex.Lock()
	defer sighupMutex.Unlock()
	sighupHandlers = make([]func(), 0, 10)
	if sighupChan == nil {
		return
	}
	signal.Stop(sighupChan)
	close(sighupChan)
}

// here we would store system loggers
var (
	loggerMap        map[string]*zerolog.Logger
	loggerMutex      sync.RWMutex
	rotateHupWriters map[string]*rotatewriter.RotateWriter

	// NOTE: after every work with loggerMap check loggerMap["system"] in this variable in code
	systemLogger *zerolog.Logger
	// this is fallback logger to use if systemLogger is nil to error stdout
	fallbackSystemLogger *zerolog.Logger
	httpLogger           *zerolog.Logger
	fallbackHTTPLogger   *zerolog.Logger
)

/*
Default log config is
for fallback cases(no section in config ets if config==nil)
plain out to stderr
 system:{
                // typos would be pluggable to make put there extra info
                // may be stdout, stderr, syslog, file
                output: "stderr",
                // for file logs,console logs: plain|json, excepts syslog
                format: "plain",
                path: "./logs/system.log",
				rotate: { // right for output=="file"
					rotate: true,
                    // rotate or reopen on ON SIGHUP (depends on files parameter below)
                    sighup: true,
                    // number of files to rotate. 0 for just to reopen
                    files: 5,
                    // this for future features
                    // null, false, or string cron description
                    cron: false,
                },
			},
*/

// CheckLogConfig checks config internals before and in SetupLog call
func CheckLogConfig(tag string, conf configuration.IConfig) error {
	if conf == nil || reflect.ValueOf(conf).IsNil() {
		return nil
	}
	output, err := conf.GetStringValue("output")
	if nil != err {
		return fmt.Errorf("No output in log config")
	}
	switch output {
	case "stdout":
	case "stderr":
	case "syslog":
	case "file":
		fname, err := conf.GetStringValue("path")
		if err != nil {
			return fmt.Errorf("Error with path variable in config %s: %v", tag, err)
		}
		if "" == fname {
			return fmt.Errorf("Error with fname variable in config %s - it must be empty", tag)
		}
		rotate, err := conf.GetBooleanValue("rotate", "rotate")
		if err != nil {
			return fmt.Errorf("Error with rotate/rotate variable in config %s: %v", tag, err)
		}
		if rotate {
			_, err = conf.GetBooleanValue("rotate", "sighup")
			if nil != err {
				return fmt.Errorf("Error getting sighup option from rotate subsection: %v", err)
			}
		}
	case "null":
	default:
	}
	return nil
}

//NullWriter writes nothing writer - to loopback logs
type NullWriter struct{}

func (*NullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// SetupLog setup logger to work with
func SetupLog(tag string, config configuration.IConfig) (logger *zerolog.Logger, err error) {
	// nedd that to solve lock or not
	//_, logger_already_set := loggerMap[tag]

	conf := config
	err = CheckLogConfig(tag, conf)
	if err != nil {
		return nil, err
	}
	if conf == nil || reflect.ValueOf(conf).IsNil() {
		conf, err = configuration.NewHJSONConfig([]byte(`{
			// typos would be pluggable to make put there extra info
			// may be stdout, stderr, syslog, file
			output: "stderr",
			// for file logs,console logs: plain|json, excepts syslog
			format: "plain",
			path: "./logs/system.log",
			rotate: { // right for output=="file"
				// rotate or reopen on ON SIGHUP (depends on files parameter below)
				sighup: true,
				// number of files to rotate. 0 for just to reopen
				files: 5,
				// this for future features
				// null, false, or string cron description
				cron: false,
			},
		}`))
		if nil != err {
			return nil, fmt.Errorf("Internal error while config creation occurred: %v", err)
		}
	}
	output, err := conf.GetStringValue("output")
	if nil != err {
		return nil, fmt.Errorf("No output in log config")
	}
	logFormat, err := conf.GetStringValue("format")
	rotateFileOnHup := false
	switch logFormat {
	case "plain":
	case "json":
	case "console":
	case "":
		logFormat = "plain"
	default:
		return nil, fmt.Errorf("Frong log formet in configuration file: %v", logFormat)
	}
	var writer io.Writer
	// to prevent type casting there
	var rwriter *rotatewriter.RotateWriter
	var levelWriter zerolog.LevelWriter
	switch output {
	case "null":
		writer = &NullWriter{}
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	case "syslog":
		// TODO add tag there, host, etc
		syslogwriter, err := syslog.New(syslog.LOG_LOCAL0, "basecms_tag")
		if nil != err {
			return nil, fmt.Errorf("Error while adding syslog")
		}
		levelWriter = zerolog.SyslogLevelWriter(syslogwriter)
	case "file":
		fname, err := conf.GetStringValue("path")
		if err != nil {
			return nil, fmt.Errorf("Error with path variable in config %s: %v", tag, err)
		}
		rotate, err := conf.GetBooleanValue("rotate", "rotate")
		if err != nil {
			return nil, fmt.Errorf("Error with rotate/rotate variable in config %s: %v", tag, err)
		}
		sighup := false
		if rotate {
			sighup, err = conf.GetBooleanValue("rotate", "sighup")
			if nil != err {
				return nil, fmt.Errorf("Error getting sighup option from rotate subsection: %v", err)
			}
		}
		// TODO: numfiles >0  and internal cron for future
		rwriter, err = rotatewriter.NewRotateWriter(fname, 0)
		if err != nil {
			return nil, fmt.Errorf("Error setup writer %s: %v", tag, err)
		}
		writer = rwriter
		rotateFileOnHup = rotate && sighup
		// TODO: want GetBooleanvalue boolean value in configuration-go
		//writer, err=rotatewriter.NewRotateWriter()
	default:
	}
	logger = nil
	// this is for case of writer defined:
	switch output {
	case "stdout":
		fallthrough
	case "stderr":
		fallthrough
	case "file":
		if writer == nil || reflect.ValueOf(writer).IsNil() {
			return nil, fmt.Errorf("No writer was creared")
		}
		switch logFormat {
		case "plain":
			l := zerolog.New(zerolog.ConsoleWriter{Out: writer, NoColor: true}).With().Timestamp().Logger()
			logger = &l
		case "console":
			l := zerolog.New(zerolog.ConsoleWriter{Out: writer, NoColor: false}).With().Timestamp().Logger()
			logger = &l
		case "json":
			l := zerolog.New(writer).With().Timestamp().Logger()
			logger = &l
		default:
			return nil, fmt.Errorf("Internal error while config creation occurred. No log format defined")
		}
	case "syslog":
		if levelWriter == nil || reflect.ValueOf(levelWriter).IsNil() {
			return nil, fmt.Errorf("No levelWriter was creared for syslog")
		}
		l := zerolog.New(levelWriter)
		logger = &l
	default:
	}

	if nil == logger {
		return nil, fmt.Errorf("Internal error no logger was created")
	}

	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	sighupMutex.Lock()
	defer sighupMutex.Unlock()
	loggerMap[tag] = logger
	switch tag {
	case "system":
		systemLogger = logger
	case "http":
		httpLogger = logger
	default:
	}
	if ("file" == output) && rotateFileOnHup {
		rotateHupWriters[tag] = rwriter
	}
	// todo: add rotation routines there
	return logger, nil
}

// GetLogger returns tagged logger to work with and write logs to
func GetLogger(tag string) *zerolog.Logger {
	// just lock for reading
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	logger, lSet := loggerMap[tag]
	if !lSet {
		return nil
	}
	return logger
}

// DropLogger drops log from our internal registry to e.g. reinit them while reload logs
func DropLogger(tag string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	delete(loggerMap, tag)
	rw, ok := rotateHupWriters[tag]
	if ok {
		rw.CloseWriteFile()
		rw = nil
	}
	delete(rotateHupWriters, tag)
	switch tag { // cleanup "fast" variables
	case "system":
		systemLogger = nil
	case "http":
		httpLogger = nil
	default:
	}
}

// SetupSighupRotationForLogs setups rotation hadlers for logs
// Call that functions when all logs are set up and configures
func SetupSighupRotationForLogs() error {
	AddSighupHandler(func() {
		loggerMutex.RLock()
		defer loggerMutex.RUnlock()
		for tag, writer := range rotateHupWriters {
			l, ok := loggerMap[tag]
			if !ok {
				continue
			}
			l.Info().Msg("Log rotation started")
			err := writer.Rotate(func() {
				l.Info().Msg("Rotation successful")
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error rotation of system log \"%s\": %v", tag, err)
				continue
			}
		}
	})
	return nil
}

// GetSystemLogger is to avoid env.GetLoggger("system") calls to minimize some work
// is system logger is not initialized out to stderr
func GetSystemLogger() *zerolog.Logger {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	if systemLogger == nil {
		return fallbackSystemLogger
	}
	return systemLogger
}

// GetHTTPLogger is to avoid env.GetLoggger("http") calls to minimize some work
func GetHTTPLogger() *zerolog.Logger {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	if (httpLogger == nil) || reflect.ValueOf(httpLogger).IsNil() {
		return fallbackHTTPLogger
	}
	return httpLogger
}

var (
	exitActions      []func()
	exitActionsMutex sync.Mutex
)

// AddExitAction adds action to our exit function(close logs, remove pid file etc. - for normal exit case)
func AddExitAction(f func()) {
	exitActionsMutex.Lock()
	defer exitActionsMutex.Unlock()
	exitActions = append(exitActions, f)
}

// Exit run exit actions in reverse order one by one and after run os.exit
// no sense to test cause exits program motherfucker
func Exit(code int) {
	exitActionsMutex.Lock()
	tmp := make([]func(), len(exitActions))
	for i, f := range exitActions {
		tmp[len(exitActions)-i-1] = f
	}
	exitActionsMutex.Unlock()
	for _, f := range tmp {
		f()
	}
	os.Exit(code)
}

var (
	// need that to control lock file normally
	fileLock     *flock.Flock
	fileLockPath string
	flMutex      sync.Mutex
)

// DropLockFile drops lock file and removes them
func DropLockFile() {
	flMutex.Lock()
	defer flMutex.Unlock()
	if fileLock != nil {
		fileLock.Unlock()
	}
	if fileLockPath != "" {
		os.Remove(fileLockPath)
	}
	fileLock = nil
	fileLockPath = ""
}

// CheckLockFileConfig checks config file if is not correct
func CheckLockFileConfig(conf configuration.IConfig) (err error) {
	if conf == nil || reflect.ValueOf(conf).IsNil() {
		return nil
	}
	b, err := conf.GetBooleanValue("lockfile")
	if err != nil {
		return fmt.Errorf("In lockfile section must present boolean lockfile variable(true or false)")
	}
	if b {
		lpath, err := conf.GetStringValue("file")
		if err != nil {
			return fmt.Errorf("In lockfile section must present string \"file\" variable with correct path for lockfile, writable by program")
		}
		if lpath == "" {
			return fmt.Errorf("Path to lock file cannot be empty")
		}
	}
	return nil
}

// SetupLockFile sets up LockFile by the given config
func SetupLockFile(conf configuration.IConfig) (err error) {
	flMutex.Lock()
	defer flMutex.Unlock()
	if (fileLock != nil) && (fileLockPath != "") {
		return nil
	}
	fileLock = nil
	if conf == nil {
		return nil
	}
	err = CheckLockFileConfig(conf)
	if err != nil {
		return err
	}
	b, err := conf.GetBooleanValue("lockfile")
	if err != nil {
		return fmt.Errorf("In lockfile section must present boolean lockfile variable(true or false)")
	}
	if b {
		fileLockPath, err = conf.GetStringValue("file")
		if err != nil {
			return fmt.Errorf("In lockfile section must present string \"file\" variable with correct path for lockfile, writable by program")
		}
		fileLock = flock.NewFlock(fileLockPath)
		locked, err := fileLock.TryLock()
		if err != nil {
			return fmt.Errorf("Error while locking lockfile %s: %v", fileLockPath, err)
		}
		if locked {
		} else {
			fileLock = nil
			fileLockPath = ""
			return fmt.Errorf("Error while locking lockfile %s\nProgram already running", fileLockPath)
		}
	}
	return nil
}

// CheckHTTPConfig to check http config part at startup
func CheckHTTPConfig(httpConfig configuration.IConfig) error {
	if httpConfig == nil || reflect.ValueOf(httpConfig).IsNil() {
		return fmt.Errorf("Http config part is not ready")
	}
	st, err := httpConfig.GetIntValue("shutdown_timeout")
	if err != nil {
		return fmt.Errorf("HTTP config must contain integer shutdown_timeout value")
	}
	if st < 0 {
		return fmt.Errorf("In HTTP config shutdown_timeout value must be zero or above zero")
	}
	sslConfig, err := httpConfig.GetSubconfig("ssl")
	if err != nil {
		return fmt.Errorf("Http config must contain subconfig ssl")
	}
	ssl, err := httpConfig.GetBooleanValue("ssl", "ssl")
	if err != nil {
		return fmt.Errorf("No ssl/ssl variable in http config")
	}
	if ssl {
		cert, err := sslConfig.GetStringValue("cert")
		if err != nil {
			return fmt.Errorf("Error in ssl section config with cert field: %v", err)
		}
		key, err := sslConfig.GetStringValue("key")
		if err != nil {
			return fmt.Errorf("Error in ssl section config with key field: %v", err)
		}
		if cert == "" {
			return fmt.Errorf("Error in ssl section config cert field is empty")
		}
		if key == "" {
			return fmt.Errorf("Error in ssl section config key field is empty")
		}
	}
	_, err = httpConfig.GetSubconfig("http2")
	if err != nil {
		return fmt.Errorf("Http config must contain subconfig http2")
	}
	b, err := httpConfig.GetBooleanValue("http2", "http2")
	if err != nil {
		return fmt.Errorf("No http2/http2 variable in http config")
	}
	if b {
		return fmt.Errorf("HTTP/2 must be disabled for now cause it is not supported")
	}
	s, err := httpConfig.GetStringValue("socket_type")
	if err != nil {
		return fmt.Errorf("No socket type in http configuration")
	}
	switch s {
	case "tcp":
		fallthrough
	case "unix":
	default:
		return fmt.Errorf("Socket type must be tcp or unix")
	}
	// address just must be
	_, err = httpConfig.GetStringValue("address")
	if err != nil {
		return fmt.Errorf("No address string in http config(host:port or path)")
	}
	_, err = httpConfig.GetStringValue("domain")
	if err != nil {
		return fmt.Errorf("No domain name string defined in config")
	}
	return nil
}

// CheckSetuidConfig checks setuid part of configuration file
func CheckSetuidConfig(setuidConfig configuration.IConfig) error {
	if setuidConfig == nil || reflect.ValueOf(setuidConfig).IsNil() {
		return fmt.Errorf("setuid config config part is absent")
	}
	setuid, err := setuidConfig.GetBooleanValue("setuid")
	if err != nil {
		return fmt.Errorf("No boolean value in setuid config section: %v", err)
	}
	if !setuid {
		return nil
	}
	user, err := setuidConfig.GetStringValue("user")
	if err != nil {
		return fmt.Errorf("No string user value in setuid config section: %v", err)
	}
	_, err = setuidConfig.GetStringValue("user")
	if err != nil {
		return fmt.Errorf("No string group value in setuid config section: %v", err)
	}
	if user == "" {
		return fmt.Errorf("In setuid section if setuid=true user must be non empty existing user")
	}
	// all o'k
	return nil
}

// CheckPidfileConfig checks pidfile subconfig
func CheckPidfileConfig(pidfileConfig configuration.IConfig) error {
	if pidfileConfig == nil || reflect.ValueOf(pidfileConfig).IsNil() {
		return fmt.Errorf("No pidfile config given")
	}
	pidfile, err := pidfileConfig.GetBooleanValue("pidfile")
	if err != nil {
		return fmt.Errorf("In pidfile config section no boolean pidfile value: %v", err)
	}
	if !pidfile {
		return nil
	}
	file, err := pidfileConfig.GetStringValue("file")
	if err != nil {
		return fmt.Errorf("In pidfile config section no string file value: %v", err)
	}
	if file == "" {
		return fmt.Errorf("In pidfile config section file value must be not empty")
	}
	return nil
}

var (
	pidMutex    sync.Mutex
	pidfilePath string
)

// SetupPidfile try setup pidfile if enabled.
func SetupPidfile(pidfileConfig configuration.IConfig) error {
	pidfile, err := pidfileConfig.GetBooleanValue("pidfile")
	if err != nil {
		return fmt.Errorf("In pidfile config section no boolean pidfile value: %v", err)
	}
	if !pidfile {
		return nil
	}
	file, err := pidfileConfig.GetStringValue("file")
	if err != nil {
		return fmt.Errorf("In pidfile config section no string file value: %v", err)
	}
	if file == "" {
		return fmt.Errorf("In pidfile config section file value must be not empty")
	}
	_, err = os.Stat(file)
	exists := true
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
		} else {
			return fmt.Errorf("Pidfile setting stat error: %v", err)
		}
	}
	if exists {
		return fmt.Errorf("Pidfile already exists")
	}
	pid := os.Getpid()
	s := strconv.Itoa(pid)
	err = ioutil.WriteFile(file, []byte(s), 0644)
	if err != nil {
		return fmt.Errorf("Error while writing pidfile occurred: %v", err)
	}
	pidMutex.Lock()
	defer pidMutex.Unlock()
	pidfilePath = file
	return nil
}

// DropPidfile deletes pid file on correct app shutdown
func DropPidfile() {
	pidMutex.Lock()
	defer pidMutex.Unlock()
	if pidfilePath == "" {
		return
	}
	err := os.Remove(pidfilePath)
	l := GetSystemLogger()
	if err != nil {
		if l != nil {
			l.Error().Err(err).Msg(err.Error())
		} else {
			fmt.Fprintf(os.Stderr, err.Error())
		}
	}
}

// CheckAppConfig checks whole cofiguration file like their initialization order and returns error if something is wrong
// NOTE: Environment must be initialized before start this function
// this function is intended to configtest commandline argument and for application startup
func CheckAppConfig(config configuration.IConfig) error {
	_env, err := GetEnvironment()
	if err != nil {
		return err
	}
	// lockfile section
	lockfileconfig, err := config.GetSubconfig(_env, "lockfile")
	if err != nil {
		if err != nil {
			switch err.(type) {
			case *configuration.ConfigItemNotFound:
				// this is normal case, do nothing
				err = nil
			default:
				return fmt.Errorf("lockfile must be section of config, not something else")
			}
		}
	}
	err = CheckLockFileConfig(lockfileconfig)
	if err != nil {
		return fmt.Errorf("Configuration error: lockfile configuraton error occurred: %v", err)
	}
	pidfileConfig, err := config.GetSubconfig(_env, "pidfile")
	if err != nil {
		if err != nil {
			switch err.(type) {
			case *configuration.ConfigItemNotFound:
				// this is normal case, do nothing
				err = nil
				return fmt.Errorf("pidfile part of config not found")
			default:
				return fmt.Errorf("pidfile must be section of config, not something else")
			}
		}
	}
	err = CheckPidfileConfig(pidfileConfig)
	if err != nil {
		return fmt.Errorf("Configuration error: pidfile configuraton error occurred: %v", err)
	}
	// setuid log check
	setuidConfig, err := config.GetSubconfig(_env, "setuid")
	if err != nil {
		if err != nil {
			switch err.(type) {
			case *configuration.ConfigItemNotFound:
				// this is normal case, do nothing
				err = nil
			default:
				return fmt.Errorf("In setuid section must present boolean lockfile variable(true or false)")
			}
		}
	}
	err = CheckSetuidConfig(setuidConfig)
	if err != nil {
		return fmt.Errorf("Configuration error: setuid section configuration error: %v", err)
	}
	// system.log check
	systemLogConf, err := config.GetSubconfig(_env, "logs", "system")
	if err != nil {
		return fmt.Errorf("Configuration error: system log configuration error: %v", err)
	}
	err = CheckLogConfig("system", systemLogConf)
	if err != nil {
		return fmt.Errorf("Configuration error: system log configuration error: %v", err)
	}
	httpLogConf, err := config.GetSubconfig(_env, "logs", "http")
	if err != nil {
		return fmt.Errorf("Configuration error: http log configuration error: %v", err)
	}
	err = CheckLogConfig("http", httpLogConf)
	if err != nil {
		return fmt.Errorf("Configuration error: http log configuration error: %v", err)
	}
	httpConf, err := config.GetSubconfig(_env, "http")
	err = CheckHTTPConfig(httpConf)
	if err != nil {
		return fmt.Errorf("Configuration error: http configuration error: %v", err)
	}
	// TODO: same with htaccess
	return nil
}

func init() {
	sighupMutex.Lock()
	defer sighupMutex.Unlock()
	_Env = ""
	_cmdFlags = nil
	sighupHandlers = make([]func(), 0, 10)
	sighupListSet = false
	sighupChan = nil
	loggerMap = map[string]*zerolog.Logger{}
	rotateHupWriters = map[string]*rotatewriter.RotateWriter{}
	systemLogger = nil
	httpLogger = nil
	exitActionsMutex.Lock()
	defer exitActionsMutex.Unlock()
	exitActions = make([]func(), 0, 20)
	flMutex.Lock()
	defer flMutex.Unlock()
	fileLock = nil
	fileLockPath = ""
	// lurk to http.go file
	httpServer = nil
	pidfilePath = ""
	fl := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true}).With().Timestamp().Logger()
	fallbackSystemLogger = &fl
	fallbackHTTPLogger = &fl
}
