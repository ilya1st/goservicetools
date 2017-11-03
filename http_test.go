package env

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ilya1st/configuration-go"
	"github.com/rs/zerolog"
)

func TestPrepareHTTPListener(t *testing.T) {
	type args struct {
		graceful   bool
		httpConfig configuration.IConfig
	}
	normalHTTPConfig, err := configuration.NewHJSONConfig([]byte(`{
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
		address: "localhost:0",
		// domain name to work with
		domain: "localhost",
	}`))
	if err != nil {
		t.Errorf("PrepareHTTPListener() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckHTTPConfig(normalHTTPConfig)
	if err != nil {
		t.Errorf("PrepareHTTPListener() error while test preparation %v. Failed run tests", err)
		return
	}
	systemLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	err = CheckLogConfig("system", systemLoggerconf)
	if err != nil {
		t.Errorf("PrepareHTTPListener() error while test preparation %v. Failed run tests", err)
		return
	}
	DropLogger("system")
	if err != nil {
		t.Errorf("PrepareHTTPListener() error while test preparation %v. Failed run tests", err)
		return
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		mustPanic bool
		preRun    func()
		postRun   func()
	}{
		{
			name:      "test without logger",
			args:      args{graceful: true, httpConfig: normalHTTPConfig},
			wantErr:   false,
			mustPanic: false,
			preRun:    func() {},
			postRun:   func() {},
		},
		{
			name:      "run with syslog and no config",
			args:      args{graceful: true, httpConfig: nil},
			wantErr:   false,
			mustPanic: true,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("PrepareHTTPListener() error while test preparation %v.", err)
					return
				}
			},
			postRun: func() {
				DropLogger("system")
				DropHTTPListener()
			},
		},
		{
			name:      "run with syslog and config and graceful + graceful env there. must fail",
			args:      args{graceful: true, httpConfig: normalHTTPConfig},
			wantErr:   true,
			mustPanic: false,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("PrepareHTTPListener() error while test preparation %v.", err)
					return
				}
				os.Setenv("GRACEFUL_HTTP_FD", "4")
			},
			postRun: func() {
				DropLogger("system")
				DropHTTPListener()
				os.Setenv("GRACEFUL_HTTP_FD", "")
			},
		},
		{
			name:      "run syslog, nongraceful, nil config",
			args:      args{graceful: false, httpConfig: nil},
			wantErr:   false,
			mustPanic: true,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("PrepareHTTPListener() error while test preparation %v.", err)
					return
				}
			},
			postRun: func() {
				DropLogger("system")
				DropHTTPListener()
			},
		},
		{
			name:      "run syslog, nongraceful, normal config",
			args:      args{graceful: false, httpConfig: normalHTTPConfig},
			wantErr:   false,
			mustPanic: false,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("PrepareHTTPListener() error while test preparation %v.", err)
					return
				}
			},
			postRun: func() {
				DropLogger("system")
				DropHTTPListener()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun()
			}
			defer func() {
				if r := recover(); r != nil {
					if !tt.mustPanic {
						t.Errorf("PrepareHTTPListener() do not must panic but does %v", r)
					}
				}
			}()
			err := PrepareHTTPListener(tt.args.graceful, tt.args.httpConfig)
			if tt.mustPanic {
				t.Errorf("PrepareHTTPListener() do must panic but does not")
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("PrepareHTTPListener() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.postRun != nil {
				tt.postRun()
			}
			DropLogger("system")
			DropHTTPListener()
		})
	}
}

func TestGetHTTPListener(t *testing.T) {
	normalHTTPConfig, err := configuration.NewHJSONConfig([]byte(`{
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
		address: "localhost:0",
		// domain name to work with
		domain: "localhost",
	}`))
	if err != nil {
		t.Errorf("GetHTTPListener() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckHTTPConfig(normalHTTPConfig)
	if err != nil {
		t.Errorf("GetHTTPListener() error while test preparation %v. Failed run tests", err)
		return
	}
	systemLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	err = CheckLogConfig("system", systemLoggerconf)
	if err != nil {
		t.Errorf("GetHTTPListener() error while test preparation %v. Failed run tests", err)
		return
	}
	DropLogger("system")
	DropHTTPListener()
	tests := []struct {
		name    string
		want    bool
		preRun  func()
		postRun func()
	}{
		{name: "just stupid run", want: false},
		{
			name: "get listener if set up",
			want: true,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("GetHTTPListener() error while test preparation env.SetupLog() reports: %v.", err)
					return
				}
				err = PrepareHTTPListener(false, normalHTTPConfig)
				if err != nil {
					t.Errorf("GetHTTPListener() error while test preparation env.PrepareHTTPListener() reports: %v.", err)
					return
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					t.Errorf("GetHTTPListener() panic error occured: %v", r)
				}
				DropLogger("system")
				DropHTTPListener()
			}()
			if tt.preRun != nil {
				tt.preRun()
			}
			got := GetHTTPListener()
			if tt.want && (got == nil || reflect.ValueOf(got).IsNil()) {
				t.Errorf("GetHTTPListener() want not nil but got nil")
			}
			if !tt.want && (got != nil && !reflect.ValueOf(got).IsNil()) {
				t.Errorf("GetHTTPListener() wanted nit but got %v", got)
			}
		})
	}
}

func TestDropHTTPListener(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "just run them"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					t.Errorf("DropHTTPListener() panic error occured: %v", r)
				}
			}()
			DropHTTPListener()
		})
	}
}

func TestGetHTTPServer(t *testing.T) {
	tests := []struct {
		name string
		want *http.Server
	}{
		{name: "Just stupid run there when all is dropped", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetHTTPServer()
			if (tt.want == nil && got != nil) || (tt.want != nil && got == nil) {
				t.Errorf("GetHTTPServer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupHTTPServer(t *testing.T) {
	normalHTTPConfig, err := configuration.NewHJSONConfig([]byte(`{
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
		address: "localhost:0",
		// domain name to work with
		domain: "localhost",
	}`))
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckHTTPConfig(normalHTTPConfig)
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	systemLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", systemLoggerconf)
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	httpLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", httpLoggerconf)
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	DropLogger("system")
	DropLogger("http")
	DropHTTPListener()
	DropHTTPServer()
	type args struct {
		httpConfig configuration.IConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		preRun  func()
		postRun func()
	}{
		{
			name:    "normal stupid run",
			args:    args{httpConfig: normalHTTPConfig},
			wantErr: false,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				_, err = SetupLog("http", httpLoggerconf)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				err = PrepareHTTPListener(false, normalHTTPConfig)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				err = SetupHTTPServer(normalHTTPConfig)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
			},
			postRun: func() {
				DropHTTPServer()
				DropHTTPListener()
				DropLogger("http")
				DropLogger("system")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					t.Errorf("SetupHTTPServer() panic  error while tests= %v", r)
				}
			}()
			if tt.preRun != nil {
				tt.preRun()
			}
			if err := SetupHTTPServer(tt.args.httpConfig); (err != nil) != tt.wantErr {
				t.Errorf("SetupHTTPServer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.postRun != nil {
				tt.postRun()
			}
		})
	}
}

func TestDropHTTPServer(t *testing.T) {
	normalHTTPConfig, err := configuration.NewHJSONConfig([]byte(`{
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
		address: "localhost:0",
		// domain name to work with
		domain: "localhost",
	}`))
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckHTTPConfig(normalHTTPConfig)
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	systemLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", systemLoggerconf)
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	httpLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", httpLoggerconf)
	if err != nil {
		t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	DropLogger("system")
	DropLogger("http")
	DropHTTPListener()
	tests := []struct {
		name    string
		preRun  func()
		postRun func()
	}{
		{name: "run in non initialized app"},
		{
			name: "Drop ready server",
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				_, err = SetupLog("http", httpLoggerconf)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				err = PrepareHTTPListener(false, normalHTTPConfig)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				err = SetupHTTPServer(normalHTTPConfig)
				if err != nil {
					t.Errorf("SetupHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
			},
			postRun: func() {
				DropHTTPListener()
				DropLogger("http")
				DropLogger("system")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					t.Errorf("SetupHTTPServer() panic  error while tests= %v", r)
				}
			}()
			if tt.preRun != nil {
				tt.preRun()
			}
			DropHTTPServer()
			if tt.postRun != nil {
				tt.postRun()
			}
		})
	}
}

func TestSetHTTPServeMux(t *testing.T) {
	normalHTTPConfig, err := configuration.NewHJSONConfig([]byte(`{
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
		address: "localhost:0",
		// domain name to work with
		domain: "localhost",
	}`))
	if err != nil {
		t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckHTTPConfig(normalHTTPConfig)
	if err != nil {
		t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
		return
	}
	systemLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", systemLoggerconf)
	if err != nil {
		t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
		return
	}
	httpLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", httpLoggerconf)
	if err != nil {
		t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
		return
	}
	DropLogger("system")
	DropLogger("http")
	DropHTTPListener()
	DropHTTPServer()
	newMux := http.NewServeMux()
	newMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// TODO: http log sample there
		fmt.Fprintf(w, "This is default server mux. See defaultAppStartSetup setting IAppStartSetup in appstart.go file. You can create you one. URI: %v", r.URL.Path)
	})
	type args struct {
		mux *http.ServeMux
	}
	tests := []struct {
		name      string
		args      args
		preRun    func()
		postRun   func()
		wantPanic bool
	}{
		{name: "noserver case", args: args{mux: newMux}, wantPanic: true},
		{
			name:      "about full start",
			args:      args{mux: newMux},
			wantPanic: false,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
					return
				}
				_, err = SetupLog("http", httpLoggerconf)
				if err != nil {
					t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
					return
				}
				err = PrepareHTTPListener(false, normalHTTPConfig)
				if err != nil {
					t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
					return
				}
				err = SetupHTTPServer(normalHTTPConfig)
				if err != nil {
					t.Errorf("SetHTTPServeMux() error while test preparation %v. Failed run tests", err)
					return
				}
			},
			postRun: func() {
				DropHTTPServer()
				DropHTTPListener()
				DropLogger("http")
				DropLogger("system")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r != nil && !tt.wantPanic {
					t.Errorf("SetHTTPServeMux() unexpectable panic occured: %v", err)
				}
			}()
			if tt.preRun != nil {
				tt.preRun()
			}
			SetHTTPServeMux(tt.args.mux)
			if tt.postRun != nil {
				tt.postRun()
			}
			if tt.wantPanic {
				t.Errorf("SetHTTPServeMux() must give panic in such case but does not")
			}
		})
	}
}

func TestStartHTTPServer(t *testing.T) {
	os.Setenv("ENv", "test")
	normalHTTPConfig, err := configuration.NewHJSONConfig([]byte(`{
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
		address: "localhost:9999",
		// domain name to work with
		domain: "localhost",
	}`))
	if err != nil {
		t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckHTTPConfig(normalHTTPConfig)
	if err != nil {
		t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	systemLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", systemLoggerconf)
	if err != nil {
		t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	httpLoggerconf, err := configuration.NewHJSONConfig([]byte(`{
		// typos would be pluggable to make put there extra info
		// may be stdout, stderr, syslog, file
		output: "stderr",
		// for file logs,console logs: plain|json|console, excepts syslog
		format: "plain",
		path: "./logs/system.log",
		rotate: { // right for output=="file"
			// do we need rotation
			rotate: true,
			// rotate or reopen on ON SIGHUP (depends on files parameter below)
			sighup: true,
		},
	}`))
	if err != nil {
		t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckLogConfig("system", httpLoggerconf)
	if err != nil {
		t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	DropLogger("system")
	DropLogger("http")
	DropHTTPListener()
	DropHTTPServer()
	newMux := http.NewServeMux()
	newMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// TODO: http log sample there
		fmt.Fprintf(w, "This is default server mux. See defaultAppStartSetup setting IAppStartSetup in appstart.go file. You can create you one. URI: %v", r.URL.Path)
	})
	tests := []struct {
		name      string
		preRun    func()
		postRun   func()
		wantPanic bool
	}{
		{
			name:      "uninitialized run. must fall in panic",
			wantPanic: true,
			preRun: func() {
				DropHTTPServer()
				DropHTTPListener()
				DropLogger("http")
				DropLogger("system")
			},
			postRun: func() {
				DropHTTPServer()
				DropHTTPListener()
				DropLogger("http")
				DropLogger("system")
			},
		},
		{
			name: "full preinit no panic", wantPanic: false,
			preRun: func() {
				_, err := SetupLog("system", systemLoggerconf)
				if err != nil {
					t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				_, err = SetupLog("http", httpLoggerconf)
				if err != nil {
					t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				err = PrepareHTTPListener(false, normalHTTPConfig)
				if err != nil {
					t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
				err = SetupHTTPServer(normalHTTPConfig)
				if err != nil {
					t.Errorf("StartHTTPServer() error while test preparation %v. Failed run tests", err)
					return
				}
			},
			postRun: func() {
				DropHTTPServer()
				DropHTTPListener()
				DropLogger("http")
				DropLogger("system")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r != nil && !tt.wantPanic {
					t.Errorf("StartHTTPServer() unexpectable panic occured: %v", err)
				}
			}()
			if tt.preRun != nil {
				tt.preRun()
			}
			StartHTTPServer()
			// need that cause there is gorotine inside
			time.Sleep(200 * time.Millisecond)
			if tt.postRun != nil {
				tt.postRun()
			}
			if tt.wantPanic {
				t.Errorf("SetHTTPServeMux() must give panic in such case but does not")
			}
		})
	}
}

func Test_httpErrorWriter_Write(t *testing.T) {
	type fields struct {
		log *zerolog.Logger
	}
	type args struct {
		p []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantN   int
		wantErr bool
	}{
		{
			name:    "just show it runs and non broken",
			fields:  fields{nil},
			args:    args{[]byte("string")},
			wantN:   6,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &httpErrorWriter{
				log: tt.fields.log,
			}
			gotN, err := l.Write(tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("httpErrorWriter.Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotN != tt.wantN {
				t.Errorf("httpErrorWriter.Write() = %v, want %v", gotN, tt.wantN)
			}
		})
	}
}
