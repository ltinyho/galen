package galen

import (
	"errors"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/ltinyho/galen/gdebugserver"
	"github.com/ltinyho/galen/glog"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ErrShowVersion Predefined errors
var (
	ErrShowVersion = errors.New("ErrShowVersion")
)

func (a *Application) handleFlagsAndEnv() error {
	var err error
	err = gdebugserver.RegisterDebugServerPFlags()
	if err != nil {
		return err
	}

	pflag.StringP("file", "f", "app.yml", "config file name")
	pflag.BoolP("version", "v", false, "show version")

	if a.preload != nil {
		if err = a.preload(); err != nil {
			glog.WithError(err).Debug("preload error")
			return err
		}
	}

	a.cmdline.Parse(glog.FilterFlags(os.Args[1:]))

	// bind pflags
	err = viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		return err
	}

	// bind env
	viper.AutomaticEnv()

	// if just show version
	if viper.GetBool("version") {
		err = showAppVersion(os.Stdout, a.name)
		if err != nil {
			return err
		}
		return ErrShowVersion
	}

	// read from config file
	viper.SetConfigFile(viper.GetString("file"))
	err = viper.ReadInConfig() // Find and read the config file

	if err != nil { // Handle errors reading the config file
		glog.WithError(err).Debug("Read from config fail, use default settings") // it is ok that we cannot read from config file
	} else {
		// watch config change
		if a.onConfigFileChanged != nil {
			viper.WatchConfig()
			viper.OnConfigChange(func(e fsnotify.Event) {
				glog.Trace("Config file changed:", e.Name)
				a.onConfigFileChanged()
			})
		}
	}
	return nil
}

// WithCmdLine set init with a timeout
func WithCmdLine(cmdline *pflag.FlagSet) AppOpts {
	return func(a *Application) {
		a.cmdline = cmdline
	}
}

// WithConfigChanged set config change handler for app
func WithConfigChanged(onConfigChange func()) AppOpts {
	return func(a *Application) {
		a.onConfigFileChanged = onConfigChange
	}
}

// WithPreload set preload func,
// preload is only used to set flags or some common setting, normally, we should use init func
func WithPreload(preload func() error) AppOpts {
	return func(a *Application) {
		a.preload = preload
	}
}
