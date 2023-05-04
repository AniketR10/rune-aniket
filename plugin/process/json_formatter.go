package process

import (
	log "github.com/sirupsen/logrus"
)

type jsonFormatter struct {
	formatter *log.JSONFormatter
}

func newJSONFormatter() log.Formatter {
	return jsonFormatter{
		formatter: &log.JSONFormatter{
			// timestamp format expected by hclog
			TimestampFormat: "2006-01-02T15:04:05.000000Z07:00",
			FieldMap: log.FieldMap{
				log.FieldKeyTime: "@timestamp",
				log.FieldKeyMsg:  "@message",
				// log.FieldKeyLevel: "@level",
			},
		},
	}
}

func (j jsonFormatter) Format(entry *log.Entry) ([]byte, error) {
	if entry.Data == nil {
		entry.Data = make(log.Fields)
	}
	// map 'warning' (logrus) to 'warn' (hclog) or otherwise
	// warning logs are printed verbatim (json).
	levelStr := entry.Level.String()
	if entry.Level == log.WarnLevel {
		levelStr = "warn"
	}
	entry.Data["@level"] = levelStr
	return j.formatter.Format(entry)
}
