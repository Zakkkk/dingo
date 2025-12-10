package lsp

import "errors"

// MultiLogger routes log calls to multiple loggers
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a logger that writes to multiple destinations
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

func (m *MultiLogger) Debugf(format string, args ...interface{}) {
	for _, l := range m.loggers {
		l.Debugf(format, args...)
	}
}

func (m *MultiLogger) Infof(format string, args ...interface{}) {
	for _, l := range m.loggers {
		l.Infof(format, args...)
	}
}

func (m *MultiLogger) Warnf(format string, args ...interface{}) {
	for _, l := range m.loggers {
		l.Warnf(format, args...)
	}
}

func (m *MultiLogger) Errorf(format string, args ...interface{}) {
	for _, l := range m.loggers {
		l.Errorf(format, args...)
	}
}

func (m *MultiLogger) Fatalf(format string, args ...interface{}) {
	for _, l := range m.loggers {
		l.Fatalf(format, args...)
	}
}

// Close closes all loggers that implement io.Closer
func (m *MultiLogger) Close() error {
	var errs []error
	for _, l := range m.loggers {
		if closer, ok := l.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
