/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package logger

// Logger is the interface for Logger types
type Logger interface {
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Debug(args ...interface{})

	Infof(fmt string, args ...interface{})
	Warnf(fmt string, args ...interface{})
	Errorf(fmt string, args ...interface{})
	Debugf(fmt string, args ...interface{})
}

// LoggerWrapper is used to wrap raw logger to match AddCallerSkip(1) of logger layer skip
type LoggerWrapper struct {
	logger Logger
}

// NewLoggerWrapper
func NewLoggerWrapper(logger Logger) *LoggerWrapper {
	return &LoggerWrapper{
		logger: logger,
	}
}

func (w *LoggerWrapper) Info(args ...interface{}) {
	w.logger.Info(args...)
}
func (w *LoggerWrapper) Warn(args ...interface{}) {
	w.logger.Warn(args...)
}
func (w *LoggerWrapper) Error(args ...interface{}) {
	w.logger.Error(args...)
}
func (w *LoggerWrapper) Debug(args ...interface{}) {
	w.logger.Debug(args...)
}

func (w *LoggerWrapper) Infof(fmt string, args ...interface{}) {
	w.logger.Infof(fmt, args...)
}
func (w *LoggerWrapper) Warnf(fmt string, args ...interface{}) {
	w.logger.Warnf(fmt, args...)
}
func (w *LoggerWrapper) Errorf(fmt string, args ...interface{}) {
	w.logger.Errorf(fmt, args...)
}
func (w *LoggerWrapper) Debugf(fmt string, args ...interface{}) {
	w.logger.Debugf(fmt, args...)
}
