package goservicetools

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	configuration "github.com/ilya1st/configuration-go"
)

func TestDefaultAppStartSetup_NeedHTTP(t *testing.T) {
	os.Setenv("ENV", "test")
	tests := []struct {
		name string
		d    *DefaultAppStartSetup
		want bool
	}{
		{
			name: "just run",
			d:    &DefaultAppStartSetup{},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.d
			if got := d.NeedHTTP(); got != tt.want {
				t.Errorf("DefaultAppStartSetup.NeedHTTP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultAppStartSetup_CommandLineHook(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		cmdFlags map[string]string
	}
	tests := []struct {
		name string
		d    *DefaultAppStartSetup
		args args
	}{
		{name: "Just stupid run them", d: &DefaultAppStartSetup{}, args: args{cmdFlags: map[string]string{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultAppStartSetup{}
			d.CommandLineHook(tt.args.cmdFlags)
		})
	}
}

func TestDefaultAppStartSetup_CheckUserConfig(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		mainconf configuration.IConfig
	}
	tests := []struct {
		name    string
		d       *DefaultAppStartSetup
		args    args
		wantErr bool
	}{
		{
			name:    "just run them, cause it does nothing for us",
			d:       &DefaultAppStartSetup{},
			args:    args{mainconf: nil},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultAppStartSetup{}
			if err := d.CheckUserConfig(tt.args.mainconf); (err != nil) != tt.wantErr {
				t.Errorf("DefaultAppStartSetup.CheckUserConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultAppStartSetup_SystemSetup(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		graceful bool
	}
	tests := []struct {
		name    string
		d       *DefaultAppStartSetup
		args    args
		wantErr bool
	}{
		{
			name:    "Just stupidly run them, cause prototype does nothing",
			d:       &DefaultAppStartSetup{},
			args:    args{graceful: false},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultAppStartSetup{}
			if err := d.SystemSetup(tt.args.graceful); (err != nil) != tt.wantErr {
				t.Errorf("DefaultAppStartSetup.SystemSetup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultAppStartSetup_HandleSignal(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		sg os.Signal
	}
	tests := []struct {
		name string
		d    *DefaultAppStartSetup
		args args
	}{
		{name: "Just run them", d: &DefaultAppStartSetup{}, args: args{sg: syscall.SIGINT}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultAppStartSetup{}
			d.HandleSignal(tt.args.sg)
		})
	}
}

func TestDefaultAppStartSetup_ConfigureHTTPServer(t *testing.T) {
	os.Setenv("ENV", "test")
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
		t.Errorf("ConfigureHTTPServer() error while test preparation %v. Failed run tests", err)
		return
	}
	err = CheckHTTPConfig(normalHTTPConfig)
	if err != nil {
		t.Errorf("HTTPServerListener() error while test preparation %v. Failed run tests", err)
		return
	}
	type args struct {
		graceful bool
	}
	tests := []struct {
		name    string
		d       *DefaultAppStartSetup
		args    args
		wantErr bool
		preRun  func()
		postRun func()
	}{
		{
			name:    "Check if it works",
			d:       &DefaultAppStartSetup{},
			args:    args{graceful: false},
			wantErr: false,
			preRun: func() {
				_, err := SetupLog("system", nil)
				if err != nil {
					t.Errorf("HTTPServerListener() error while test preparation %v. Failed run tests", err)
					return
				}
				err = PrepareHTTPListener(false, normalHTTPConfig)
				if err != nil {
					panic(err)
				}
				err = SetupHTTPServer(normalHTTPConfig)
				if err != nil {
					panic(err)
				}
			},
			postRun: func() {
				DropHTTPServer()
				DropHTTPListener()
				DropLogger("system")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r != nil {
					t.Errorf("PrepareHTTPConfigureHTTPServerListener() error while running test: %v", r)
					return
				}
			}()
			d := tt.d
			if tt.preRun != nil {
				tt.preRun()
			}
			if err := d.ConfigureHTTPServer(tt.args.graceful); (err != nil) != tt.wantErr {
				t.Errorf("DefaultAppStartSetup.ConfigureHTTPServer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.postRun != nil {
				tt.postRun()
			}
		})
	}
}

func TestDefaultAppStartSetup_SystemStart(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		graceful bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "just run them", args: args{graceful: false}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultAppStartSetup{}
			if err := d.SystemStart(tt.args.graceful); (err != nil) != tt.wantErr {
				t.Errorf("DefaultAppStartSetup.SystemStart() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultAppStartSetup_SystemShutdown(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		graceful bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "Just run", args: args{graceful: false}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultAppStartSetup{}
			if err := d.SystemShutdown(tt.args.graceful); (err != nil) != tt.wantErr {
				t.Errorf("DefaultAppStartSetup.SystemShutdown() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestDefaultAppStartSetup_SetupOwnExtraFiles(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		cmd *exec.Cmd
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "just run them", args: args{cmd: &exec.Cmd{}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DefaultAppStartSetup{}
			if err := d.SetupOwnExtraFiles(tt.args.cmd); (err != nil) != tt.wantErr {
				t.Errorf("DefaultAppStartSetup.SetupOwnExtraFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppStart(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
	}
	// all the test for now here would with default serve mux
	tests := []struct {
		name         string
		args         args
		wantExitCode int
		wantErr      bool
		preRun       func()
		postRun      func()
	}{
		{
			name:         "normal start",
			args:         args{},
			wantExitCode: 0,
			wantErr:      false,
			preRun: func() {
				os.Chdir("../..")
				_, err := configuration.GetConfigInstance("main", "HJSON", "./conf/config.hjson")
				if err != nil {
					t.Errorf("AppStart() = error while preinit %v", err)
				}
			},
			postRun: func() {
				time.Sleep(time.Millisecond * 200)
				AppStop(false, nil)
				os.Chdir("libraries/env")
				_Env = ""
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun()
			}
			gotExitCode, err := AppStart(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("AppStart() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotExitCode != tt.wantExitCode {
				t.Errorf("AppStart() = %v, want %v", gotExitCode, tt.wantExitCode)
			}
			if tt.postRun != nil {
				tt.postRun()
			}
		})
	}
}

func TestAppStop(t *testing.T) {
	os.Setenv("ENV", "test")
	type args struct {
		graceful bool
		sd       *SetuidData
	}
	tests := []struct {
		name         string
		args         args
		wantExitCode int
		wantErr      bool
		preRun       func()
		postRun      func()
	}{
		{
			name:         "normal stop",
			args:         args{false, nil},
			wantExitCode: 0,
			wantErr:      false,
			preRun: func() {
				GetEnvironment(true, "test")
				os.Chdir("../..")
				_, err := configuration.GetConfigInstance("main", "HJSON", "./conf/config.hjson")
				if err != nil {
					t.Errorf("AppStart() = error while preinit %v", err)
				}
				AppStart(nil)
				time.Sleep(time.Millisecond * 200)
			},
			postRun: func() {
				_Env = ""
				os.Chdir("libraries/env")
			},
		},
	}
	for _, tt := range tests {
		if tt.preRun != nil {
			tt.preRun()
		}
		t.Run(tt.name, func(t *testing.T) {
			gotExitCode, err := AppStop(tt.args.graceful, tt.args.sd)
			if (err != nil) != tt.wantErr {
				t.Errorf("AppStop() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotExitCode != tt.wantExitCode {
				t.Errorf("AppStop() = %v, want %v", gotExitCode, tt.wantExitCode)
			}
		})
		if tt.postRun != nil {
			tt.postRun()
		}
	}
}
