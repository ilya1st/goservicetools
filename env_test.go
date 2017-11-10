package goservicetools

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ilya1st/configuration-go"
)

var basePackageDir string

func init() { // cause all the tests need logs directory for testing
	_, filename, _, _ := runtime.Caller(0)
	basePackageDir = path.Dir(filename)
	os.Chdir(basePackageDir)
}
func TestValidateEnv(t *testing.T) {
	type args struct {
		str string
	}
	type teststruct struct {
		name    string
		args    args
		wantErr bool
	}
	tests := []teststruct{
		teststruct{
			name:    "failure test",
			args:    args{str: "fuckup fuckup fuckup"},
			wantErr: true,
		},
		teststruct{
			name:    "normal prod value",
			args:    args{str: "prod"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateEnv(tt.args.str); (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetEnvironment(t *testing.T) {
	type args struct {
		sl []interface{}
	}
	type teststruct struct {
		name         string
		args         args
		wantEnv      string
		wantErr      bool
		preStartCode func()
	}
	tests := []teststruct{
		teststruct{
			name:    "first run with empty args and no enb",
			args:    args{sl: []interface{}{}},
			wantEnv: "",
			wantErr: true,
			preStartCode: func() {
				_Env = ""
				os.Setenv("ENV", "")
			},
		},
		teststruct{
			name:    "first run with wrong env variable",
			args:    args{sl: []interface{}{}},
			wantEnv: "",
			wantErr: true,
			preStartCode: func() {
				_Env = ""
				os.Setenv("ENV", "fuckup")
			},
		},
		teststruct{
			name:    "first run with right env variable",
			args:    args{sl: []interface{}{}},
			wantEnv: "prod",
			wantErr: false,
			preStartCode: func() {
				_Env = ""
				os.Setenv("ENV", "prod")
			},
		},
		teststruct{
			name:    "first run with wrong first argument type",
			args:    args{sl: []interface{}{""}},
			wantEnv: "",
			wantErr: true,
			preStartCode: func() {
				_Env = ""
			},
		},
		teststruct{
			name:    "first run with wrong second argument type",
			args:    args{sl: []interface{}{true, false}},
			wantEnv: "",
			wantErr: true,
			preStartCode: func() {
				_Env = ""
			},
		},
		teststruct{
			name:    "first run: right args, wrong env ",
			args:    args{sl: []interface{}{true, "fuck"}},
			wantEnv: "",
			wantErr: true,
			preStartCode: func() {
				_Env = ""
			},
		},
		teststruct{
			name:    "first run, right args, right env",
			args:    args{sl: []interface{}{true, "prod"}},
			wantEnv: "prod",
			wantErr: false,
			preStartCode: func() {
				_Env = ""
			},
		},
		teststruct{
			name:    "second run without args, variable teoretically set by previous run",
			args:    args{sl: []interface{}{}},
			wantEnv: "prod",
			wantErr: false,
			preStartCode: func() {
				_Env = ""
				os.Setenv("ENV", "prod")
				GetEnvironment()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if nil != tt.preStartCode {
				tt.preStartCode()
			}
			gotEnv, err := GetEnvironment(tt.args.sl...)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotEnv != tt.wantEnv {
				t.Errorf("GetEnvironment() = %v, want %v", gotEnv, tt.wantEnv)
			}
		})
	}
}

func TestGetCommandLineFlags(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "simple default case",
			want: map[string]string{"config": "./conf/config.hjson"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// yes we will run them with nil argument
			if got := GetCommandLineFlags(nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCommandLineFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddSighupHandler(t *testing.T) {
	handlerCalled := false
	sighandler := func() {
		handlerCalled = true
	}
	t.Run("adding function", func(t *testing.T) {
		l1 := len(sighupHandlers)
		AddSighupHandler(sighandler)
		if l1 == len(sighupHandlers) {
			t.Errorf("AddSighupHandler() wrong behaviour with internal structure. Handler was not added")
		}
		handlerCalled = false
		sighupHandlers[l1]()
		if !handlerCalled {
			t.Errorf("AddSighupHandler() wrong behaviour with internal structure. Handler was not added")
		}
		// cleanup
		sighupHandlers = make([]func(), 0, 10)
	})
}

func TestSetupSighupHandlers(t *testing.T) {
	var hMutex sync.Mutex
	handlerCalled := false
	sighandler := func() {
		hMutex.Lock()
		defer hMutex.Unlock()
		handlerCalled = true
	}
	t.Run("adding function", func(t *testing.T) {
		SetupSighupHandlers()
		AddSighupHandler(sighandler)
		syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
		time.Sleep(time.Millisecond * 100)
		hMutex.Lock()
		if !handlerCalled {
			t.Errorf("SetupSighupHandlers() and AddSighupHandler() do not setup handlers to run SIGHUP correctly")
		}
		hMutex.Unlock()
		// cleanup
		CleanupSighupHandlers()
	})
}

func TestCleanupSighupHandlers(t *testing.T) {
	t.Run("Just call them. At all function need manual tests", func(t *testing.T) {
		CleanupSighupHandlers()
	})
}

func clenupLogs() {
	files, err := ioutil.ReadDir("./logs")
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if (f.Name() == ".gitignore") || (f.Name() == "README") {
			continue
		}
		os.Remove(path.Join("logs", f.Name()))
	}
	os.Remove("logs/test.log")
}
func TestSetupLog(t *testing.T) {
	type args struct {
		tag    string
		config configuration.IConfig
	}
	tests := []struct {
		name       string
		args       args
		wantLogger bool
		wantErr    bool
		cleanUp    func()
	}{
		{
			name:       "Default config test",
			args:       args{tag: "system", config: nil},
			wantLogger: true,
			wantErr:    false,
			cleanUp: func() {
				DropLogger("system")
			},
		},
		{
			name: "Good log file test",
			args: args{
				tag: "system",
				config: func() configuration.IConfig {
					c, _ := configuration.NewHJSONConfig([]byte(`{
						// typos would be pluggable to make put there extra info
						// may be stdout, stderr, syslog, file
						output: "file",
						// for file logs,console logs: plain|json, excepts syslog
						format: "plain",
						path: "./logs/test.log",
						rotate: { // right for output=="file"
							rotate: true,
							// rotate or reopen on ON SIGHUP (depends on files parameter below)
							sighup: true,
						},
					}`))
					return c
				}(),
			},
			wantLogger: true,
			wantErr:    false,
			cleanUp: func() {
				DropLogger("system")
				clenupLogs()
			},
		},
		{
			name: "Bad log file test",
			args: args{
				tag: "system",
				config: func() configuration.IConfig {
					c, _ := configuration.NewHJSONConfig([]byte(`{
						// typos would be pluggable to make put there extra info
						// may be stdout, stderr, syslog, file
						output: "file",
						// for file logs,console logs: plain|json, excepts syslog
						format: "plain",
						// specially make error
						path: "",
						rotate: { // right for output=="file"
							rotate: true,
							// rotate or reopen on ON SIGHUP (depends on files parameter below)
							sighup: true,
						},
					}`))
					return c
				}(),
			},
			wantLogger: false,
			wantErr:    true,
			cleanUp: func() {
				DropLogger("system")
				clenupLogs()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLogger, err := SetupLog(tt.args.tag, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetupLog() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantLogger && (gotLogger == nil) {
				t.Errorf("SetupLog() no logger was returned wwhile no error retrieved")
			}
			if nil != tt.cleanUp {
				tt.cleanUp()
			}
		})
	}
}

func TestGetLogger(t *testing.T) {
	type args struct {
		tag string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		preRun  func()
		cleanUp func()
	}{
		{
			name: "no logger yet",
			args: args{tag: "system"},
			want: false,
		},
		{
			name: "Logger defined before",
			args: args{tag: "system"},
			want: true,
			preRun: func() {
				SetupLog("system", nil)
			},
			cleanUp: func() {
				DropLogger("system")
				clenupLogs()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun()
			}
			if got := GetLogger(tt.args.tag); tt.want && (got == nil) {
				t.Errorf("GetLogger() in this keys must return logger")
				return
			}
			if got := GetLogger(tt.args.tag); !tt.want && (got != nil) {
				t.Errorf("GetLogger() in this keys must not return logger but returns")
			}
			if tt.cleanUp != nil {
				tt.cleanUp()
			}
		})
	}
}

func TestDropLogger(t *testing.T) {
	type args struct {
		tag string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "It just runs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DropLogger(tt.args.tag)
		})
	}
}

func TestSetupSighupRotationForLogs(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		preRun  func()
		cleanUp func()
	}{
		{
			name:    "Just run no logs",
			wantErr: false,
			cleanUp: func() {
				if len(sighupHandlers) == 0 {
					t.Errorf("SetupSighupRotationForLogs() rotation handler was not added")
				}
			},
		},
		{
			name:    "Add file log and try",
			wantErr: false,
			preRun: func() {
				_, err := SetupLog("system", func() configuration.IConfig {
					c, err := configuration.NewHJSONConfig([]byte(`{
						// typos would be pluggable to make put there extra info
						// may be stdout, stderr, syslog, file
						output: "file",
						// for file logs,console logs: plain|json, excepts syslog
						format: "plain",
						// specially make error
						path: "./logs/test.log",
						rotate: { // right for output=="file"
							rotate: true,
							// rotate or reopen on ON SIGHUP (depends on files parameter below)
							sighup: true,
						},
					}`))
					if err != nil {
						t.Errorf("SetupSighupRotationForLogs() error prepare rotating config: %v", err)
					}
					return c
				}())
				if err != nil {
					t.Errorf("SetupSighupRotationForLogs() error prepare rotating config: %v", err)
				}
			},
			cleanUp: func() {
				if len(sighupHandlers) == 0 {
					t.Errorf("SetupSighupRotationForLogs() rotation handler was not added")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if nil != tt.preRun {
				tt.preRun()
			}
			if err := SetupSighupRotationForLogs(); (err != nil) != tt.wantErr {
				t.Errorf("SetupSighupRotationForLogs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// clean shit
			if nil != tt.cleanUp {
				tt.cleanUp()
			}
			CleanupSighupHandlers()
			clenupLogs()
		})
	}
}

func TestGetSystemLogger(t *testing.T) {
	tests := []struct {
		name    string
		want    bool
		preRun  func()
		postRun func()
	}{
		{
			name: "no system log configured",
			// cause it would return fallbackSystemLogger
			want: true,
			preRun: func() {
				DropLogger("system")
			},
		},
		{
			name: "system log ready",
			want: true,
			preRun: func() {
				_, err := SetupLog("system", func() configuration.IConfig {
					c, err := configuration.NewHJSONConfig([]byte(`{
						// typos would be pluggable to make put there extra info
						// may be stdout, stderr, syslog, file
						output: "file",
						// for file logs,console logs: plain|json, excepts syslog
						format: "plain",
						// specially make error
						path: "./logs/test.log",
						rotate: { // right for output=="file"
							rotate: true,
							// rotate or reopen on ON SIGHUP (depends on files parameter below)
							sighup: true,
						},
					}`))
					if err != nil {
						t.Errorf("GetSystemLogger() error prepare rotating config: %v", err)
					}
					return c
				}())
				if err != nil {
					t.Errorf("GetSystemLogger() error prepare rotating config: %v", err)
				}
			},
			postRun: func() {
				DropLogger("system")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun()
			}
			got := GetSystemLogger()
			if got == nil && tt.want {
				t.Errorf("GetSystemLogger() = %v, want not nil", got)
			}
			if got != nil && !tt.want {
				t.Errorf("GetSystemLogger() = %v, want nil", got)
			}
			if tt.postRun != nil {
				tt.postRun()
			}
			clenupLogs()
		})
	}
}

func TestGetHTTPLogger(t *testing.T) {
	tests := []struct {
		name    string
		want    bool
		preRun  func()
		postRun func()
	}{
		{
			name: "no http log configured",
			// yes, cause there would be fallback logger there
			want: true,
			preRun: func() {
				DropLogger("http")
			},
		},
		{
			name: "system log ready",
			want: true,
			preRun: func() {
				_, err := SetupLog("http", func() configuration.IConfig {
					c, err := configuration.NewHJSONConfig([]byte(`{
						// typos would be pluggable to make put there extra info
						// may be stdout, stderr, syslog, file
						output: "file",
						// for file logs,console logs: plain|json, excepts syslog
						format: "plain",
						// specially make error
						path: "./logs/test.log",
						rotate: { // right for output=="file"
							rotate: true,
							// rotate or reopen on ON SIGHUP (depends on files parameter below)
							sighup: true,
						},
					}`))
					if err != nil {
						t.Errorf("GetHTTPLogger() error prepare rotating config: %v", err)
					}
					return c
				}())
				if err != nil {
					t.Errorf("GetHTTPLogger() error prepare rotating config: %v", err)
				}
			},
			postRun: func() {
				DropLogger("http")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun()
			}
			got := GetHTTPLogger()
			if got == nil && tt.want {
				t.Errorf("GetHTTPLogger() = %v, want not nil", got)
			}
			if got != nil && !tt.want {
				t.Errorf("GetHTTPLogger() = %v, want nil", got)
			}
			if tt.postRun != nil {
				tt.postRun()
			}
			clenupLogs()
		})
	}
}

func TestAddExitAction(t *testing.T) {
	type args struct {
		f func()
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "just run",
			args: args{f: func() {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitActionsMutex.Lock()
			l := len(exitActions)
			exitActionsMutex.Unlock()
			AddExitAction(tt.args.f)
			exitActionsMutex.Lock()
			defer exitActionsMutex.Unlock()
			if l+1 != len(exitActions) {
				t.Errorf("AddExitAction(): number of callbacks does not grow")
			}
		})
	}
}

func TestCheckLogConfig(t *testing.T) {
	type args struct {
		tag  string
		conf configuration.IConfig
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		beforeFunc func()
		cleanUp    func()
	}{
		{
			name:    "Nil config",
			args:    args{tag: "system", conf: nil},
			wantErr: false,
		},
		{
			name: "Wrong config - empty file",
			args: args{tag: "system", conf: func() configuration.IConfig {
				c, _ := configuration.NewHJSONConfig(map[string]interface{}{})
				return c
			}()},
			wantErr: true,
		},
		{
			name: "Wrong config - empty file",
			args: args{tag: "system", conf: func() configuration.IConfig {
				c, _ := configuration.NewHJSONConfig([]byte(`{
					// typos would be pluggable to make put there extra info
					// may be stdout, stderr, syslog, file
					output: "file",
					// for file logs,console logs: plain|json, excepts syslog
					format: "plain",
					// specially make error
					path: "./logs/test.log",
					rotate: { // right for output=="file"
						rotate: true,
						// rotate or reopen on ON SIGHUP (depends on files parameter below)
						sighup: true,
					},
				}`))
				return c
			}()},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.beforeFunc != nil {
				tt.beforeFunc()
			}
			if err := CheckLogConfig(tt.args.tag, tt.args.conf); (err != nil) != tt.wantErr {
				t.Errorf("CheckLogConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.cleanUp != nil {
				tt.cleanUp()
			}
		})
	}
}

func TestCheckLockFileConfig(t *testing.T) {
	type args struct {
		conf configuration.IConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Nil config",
			args:    args{conf: nil},
			wantErr: false,
		},
		{
			name: "Bullshit config",
			args: args{
				conf: func() configuration.IConfig {
					c, _ := configuration.NewHJSONConfig([]byte(`{some:"fake"}`))
					return configuration.IConfig(c)
				}(),
			},
			wantErr: true,
		},
		{
			name: "normal config",
			args: args{
				conf: func() configuration.IConfig {
					c, err := configuration.NewHJSONConfig([]byte(`{
						// enabled or disabled to run
						lockfile: true,
						file:"./logs/basecms.lock"
					}`))
					fmt.Printf("err err err %#v %#v %v\n", c, err, c == nil)
					return c
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckLockFileConfig(tt.args.conf); (err != nil) != tt.wantErr {
				t.Errorf("CheckLockFileConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDropLockFile(t *testing.T) {
	tests := []struct {
		name    string
		preRun  func()
		postRun func()
	}{
		// TODO: Add test cases.
		{name: "simple run"},
		{
			name: "run with lock file",
			preRun: func() {
				SetupLockFile(func() configuration.IConfig {
					c, err := configuration.NewHJSONConfig([]byte(`{
						// enabled or disabled to run
						lockfile: true,
						file:"./logs/basecms.lock"
					}`))
					fmt.Printf("err err err %#v %#v %v\n", c, err, c == nil)
					return c
				}())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun()
			}
			DropLockFile()
			if tt.postRun != nil {
				tt.postRun()
			}
		})
	}
}
func TestCheckHTTPConfig(t *testing.T) {
	type args struct {
		httpConfig configuration.IConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "nil http config",
			args:    args{httpConfig: nil},
			wantErr: true,
		},
		{
			name: "normal http config",
			args: args{
				httpConfig: func() configuration.IConfig {
					conf, err := configuration.NewHJSONConfig([]byte(`{
						shutdown_timeout: 5000,
						ssl: { // section for future
							ssl: false
						},
						http2: {//section for future
							http2: false
						},
						// may be unix or tcp
						socket_type: "tcp",
						// host:port, ip:port
						address: "localhost:8080",
						// domain name to work with
						domain: "localhost",
					}`))
					if err != nil {
						return nil
					}
					return conf
				}(),
			},
			wantErr: false,
		},
		{
			name: "abnormal socket type",
			args: args{
				httpConfig: func() configuration.IConfig {
					conf, err := configuration.NewHJSONConfig([]byte(`{
						shutdown_timeout: 5000,
						ssl: { // section for future
							ssl: false
						},
						http2: {//section for future
							http2: false
						},
						// may be unix or tcp
						socket_type: "anal",
						// host:port, ip:port
						address: "localhost:8080",
						// domain name to work with
						domain: "localhost",
					}`))
					if err != nil {
						return nil
					}
					return conf
				}(),
			},
			wantErr: true,
		},
		{
			name: "wrong configuration contents",
			args: args{
				httpConfig: func() configuration.IConfig {
					conf, err := configuration.NewHJSONConfig([]byte(`{
						// bullshit config here
						aaaa:"bbb"
					}`))
					if err != nil {
						return nil
					}
					return conf
				}(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckHTTPConfig(tt.args.httpConfig); (err != nil) != tt.wantErr {
				t.Errorf("CheckHTTPConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckSetuidConfig(t *testing.T) {
	type args struct {
		setuidConfig configuration.IConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "nil config", args: args{setuidConfig: nil}, wantErr: true},
		{
			name: "bullshit config",
			args: args{setuidConfig: func() configuration.IConfig {
				// yes we assume all ok here
				// nothing there
				conf, err := configuration.NewHJSONConfig([]byte(`{test:{}}`))
				if err != nil {
					return nil
				}
				return conf
			}()},
			wantErr: true,
		},
		{
			name: "normal config with setuid false",
			args: args{setuidConfig: func() configuration.IConfig {
				// yes we assume all ok here
				// nothing there
				conf, err := configuration.NewHJSONConfig([]byte(`{
					// true if setuid at startup enabled, false if not
					setuid: false,
					user: "www-data",
					group: "www-data"
				}`))
				if err != nil {
					return nil
				}
				return conf
			}()},
			wantErr: false,
		},
		{
			name: "normal config with setuid true",
			args: args{setuidConfig: func() configuration.IConfig {
				// yes we assume all ok here
				// nothing there
				conf, err := configuration.NewHJSONConfig([]byte(`{
					// true if setuid at startup enabled, false if not
					setuid: true,
					user: "www-data",
					group: "www-data"
				}`))
				if err != nil {
					return nil
				}
				return conf
			}()},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckSetuidConfig(tt.args.setuidConfig); (err != nil) != tt.wantErr {
				t.Errorf("CheckSetuidConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPidfileConfig(t *testing.T) {
	normalPidfileConfig, err := configuration.NewHJSONConfig([]byte(`{
		// if pidfile enabled
		pidfile: true,
		file:"./logs/pidfile.pid"
	}`))
	if err != nil {
		t.Errorf("CheckPidfileConfig() error while prepare to tests: %v", err)
		return
	}
	badConfig, err := configuration.NewHJSONConfig([]byte(`{
		// if pidfile enabled
		bullshit: "here"
	}`))
	if err != nil {
		t.Errorf("CheckPidfileConfig() error while prepare to tests: %v", err)
		return
	}
	type args struct {
		pidfileConfig configuration.IConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "nil config", args: args{nil}, wantErr: true},
		{name: "bad config", args: args{badConfig}, wantErr: true},
		{name: "normal config", args: args{normalPidfileConfig}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckPidfileConfig(tt.args.pidfileConfig); (err != nil) != tt.wantErr {
				t.Errorf("CheckPidfileConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetupPidfile(t *testing.T) {
	normalPidfileConfigOn, err := configuration.NewHJSONConfig([]byte(`{
		// if pidfile enabled
		pidfile: true,
		file:"./logs/testpidfile.pid"
	}`))
	if err != nil {
		t.Errorf("CheckPidfileConfig() error while prepare to tests: %v", err)
		return
	}
	normalPidfileConfigOff, err := configuration.NewHJSONConfig([]byte(`{
		// if pidfile enabled
		pidfile: false,
		file:"./logs/testpidfile.pid"
	}`))
	if err != nil {
		t.Errorf("CheckPidfileConfig() error while prepare to tests: %v", err)
		return
	}
	testpidfilename := "./logs/testpidfile.pid"
	pidfileExists := func() (bool, error) {
		st, err := os.Stat(testpidfilename)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		return !st.IsDir(), nil
	}
	cleanupFunc := func() error {
		ex, err := pidfileExists()
		if err != nil {
			return err
		}
		if ex {
			err = os.Remove(testpidfilename)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err != nil {
		t.Errorf("CheckPidfileConfig() error while prepare to tests: %v", err)
		return
	}
	type args struct {
		pidfileConfig configuration.IConfig
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		fileCheck func()
	}{
		{
			name:    "disabled pidfile",
			args:    args{normalPidfileConfigOff},
			wantErr: false,
			fileCheck: func() {
				e, err := pidfileExists()
				if err != nil {
					t.Errorf("CheckPidfileConfig() error while test check: %v", err)
					return
				}
				if e {
					t.Errorf("CheckPidfileConfig() pidfile exists and this is wrong")
					return
				}
			},
		},
		{
			name:    "enabled pidfile",
			args:    args{normalPidfileConfigOn},
			wantErr: false,
			fileCheck: func() {
				e, err := pidfileExists()
				if err != nil {
					t.Errorf("CheckPidfileConfig() error while test check: %v", err)
					return
				}
				if !e {
					t.Errorf("CheckPidfileConfig() pidfile not exists and this is wrong")
					return
				}
				cnt, err := ioutil.ReadFile("./logs/testpidfile.pid")
				if err != nil {
					t.Errorf("CheckPidfileConfig() error while test check: %v", err)
					return
				}
				pid, err := strconv.ParseInt(string(cnt), 10, 32)
				if err != nil {
					t.Errorf("CheckPidfileConfig() error while test check pid file contents: %v", err)
					return
				}
				myPid := os.Getpid()
				if myPid != int(pid) {
					t.Errorf("CheckPidfileConfig() pid in pid file: %v and my pid %v are different", pid, myPid)
					return
				}
				// check internal variable
				if pidfilePath != testpidfilename {
					t.Errorf("Internal variable env.pidfilePath is %v not equal test path %v", pidfilePath, testpidfilename)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// internal package variable
			pidfilePath = ""
			cleanupFunc()
			err := SetupPidfile(tt.args.pidfileConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetupPidfile() error = %v, wantErr %v", err, tt.wantErr)
				cleanupFunc()
				return
			}
			if tt.fileCheck != nil {
				tt.fileCheck()
			}
			cleanupFunc()
		})
	}
}

func TestDropPidfile(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "Just stupdi run to check no panics, etc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DropPidfile()
		})
	}
}

func TestCheckAppConfig(t *testing.T) {
	type args struct {
		config configuration.IConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		preRun  func()
	}{
		{
			name: "bullshit fake config",
			preRun: func() {
				GetEnvironment(true, "test")
			},
			args: args{
				config: func() configuration.IConfig {
					// yes we assume all ok here
					// nothing there
					conf, err := configuration.NewHJSONConfig([]byte(`{test:{}}`))
					if err != nil {
						return nil
					}
					return conf
				}(),
			},
			wantErr: true,
		},
		{
			name: "Normal(possibly) config from distribution",
			preRun: func() {
				GetEnvironment(true, "test")
			},
			args: args{
				config: func() configuration.IConfig {
					// yes we assume all ok here

					conf, err := configuration.GetConfigInstance("main", "HJSON", "./conf/config.hjson")
					if err != nil {
						return nil
					}
					return conf
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun()
			}
			if err := CheckAppConfig(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("CheckAppConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNullWriter_Write(t *testing.T) {
	type args struct {
		p []byte
	}
	tests := []struct {
		name    string
		n       *NullWriter
		args    args
		wantN   int
		wantErr bool
	}{
		{
			name:    "Just run",
			n:       &NullWriter{},
			args:    args{p: []byte(`test`)},
			wantN:   4,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NullWriter{}
			gotN, err := n.Write(tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("NullWriter.Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotN != tt.wantN {
				t.Errorf("NullWriter.Write() = %v, want %v", gotN, tt.wantN)
			}
		})
	}
}
