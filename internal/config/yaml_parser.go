// Copyright 2021 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type LogSeverity string

const (
	TRACE   LogSeverity = "TRACE"
	DEBUG   LogSeverity = "DEBUG"
	INFO    LogSeverity = "INFO"
	WARNING LogSeverity = "WARNING"
	ERROR   LogSeverity = "ERROR"
	OFF     LogSeverity = "OFF"

	parseConfigFileErrMsgFormat = "error parsing config file: %v"
)

func IsValidLogSeverity(severity LogSeverity) bool {
	switch severity {
	case
		TRACE,
		DEBUG,
		INFO,
		WARNING,
		ERROR,
		OFF:
		return true
	}
	return false
}

func IsValidLogRotateConfig(config LogRotateConfig) error {
	if config.MaxFileSizeMB <= 0 {
		return fmt.Errorf("max-file-size-mb should be atleast 1")
	}
	if config.FileCount <= 0 {
		return fmt.Errorf("file-count should be atleast 1")
	}
	return nil
}

func ParseConfigFile(fileName string) (mountConfig *MountConfig, err error) {
	mountConfig = NewMountConfig()

	if fileName == "" {
		return
	}

	buf, err := os.ReadFile(fileName)
	if err != nil {
		err = fmt.Errorf("error reading config file: %w", err)
		return
	}

	// Ensure error is thrown when unexpected configs are passed in config file.
	// Ref: https://github.com/go-yaml/yaml/issues/602#issuecomment-623485602
	decoder := yaml.NewDecoder(bytes.NewReader(buf))
	decoder.KnownFields(true)
	if err = decoder.Decode(mountConfig); err != nil {
		// Decode returns EOF in case of empty config file.
		if err == io.EOF {
			return mountConfig, nil
		}
		return mountConfig, fmt.Errorf(parseConfigFileErrMsgFormat, err)
	}

	// convert log severity to upper-case
	mountConfig.LogConfig.Severity = LogSeverity(strings.ToUpper(string(mountConfig.LogConfig.Severity)))
	if !IsValidLogSeverity(mountConfig.LogConfig.Severity) {
		err = fmt.Errorf("error parsing config file: log severity should be one of [trace, debug, info, warning, error, off]")
		return
	}

	if err = IsValidLogRotateConfig(mountConfig.LogConfig.LogRotateConfig); err != nil {
		err = fmt.Errorf(parseConfigFileErrMsgFormat, err)
		return
	}

	return
}
