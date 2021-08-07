package glog

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
)

const jsonDataKey = "args"

type JSONFormatter struct {
	TruncateCallerPath bool
	CallerPathStrip    bool
	// TimestampFormat sets the format used for marshaling timestamps.
	TimestampFormat string

	// DisableTimestamp allows disabling automatic timestamps in output
	DisableTimestamp bool

	// DisableHTMLEscape allows disabling html escaping in output
	DisableHTMLEscape bool

	// DataKey allows users to put all the log entry parameters into a nested dictionary at a given key.
	DisableDataKey bool
	DataKey        string
	logrus.JSONFormatter
	jsonFormat *logrus.JSONFormatter
	once       sync.Once
}

func (f *JSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	if f.jsonFormat == nil {
		f.once.Do(func() {
			if f.jsonFormat == nil {
				timestampFormat := f.TimestampFormat
				if timestampFormat == "" {
					timestampFormat = longTimeStamp
				}
				if f.DisableDataKey {
					f.DataKey = ""
				} else if f.DataKey == "" {
					f.DataKey = jsonDataKey
				}
				f.jsonFormat = &logrus.JSONFormatter{
					TimestampFormat:   timestampFormat,
					DisableTimestamp:  f.DisableTimestamp,
					DisableHTMLEscape: f.DisableHTMLEscape,
					DataKey:           f.DataKey,
					FieldMap:          f.FieldMap,
					CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
						return f.callerPrettier(frame)
					},
					PrettyPrint: f.PrettyPrint,
				}
			}
		})
	}
	return f.jsonFormat.Format(entry)
}

func (f *JSONFormatter) callerPrettier(frame *runtime.Frame) (function string, file string) {
	if f.TruncateCallerPath {
		if !f.CallerPathStrip {
			return frame.Function, fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line)
		} else {
			path := stripPathAuto(frame.File)
			return frame.Function, fmt.Sprintf("%s:%d", path, frame.Line)
		}
	} else {
		return f.JSONFormatter.CallerPrettyfier(frame)
	}
}

var _InitJSONFormat = func() interface{} {
	registeFormatter("json", reflect.TypeOf(JSONFormatter{}))
	return nil
}()
