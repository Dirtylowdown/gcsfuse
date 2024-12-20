// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/bitfield/script"
)

type multiReadConfig struct {
	fileIOConcurrency   int64
	maxInflightRequests int64
	numConfig           int64
	path                string
}

func multiReadBenchmark(wd string, config *multiReadConfig) (string, error) {
	cd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if err := os.Chdir(wd); err != nil {
		defer func() { _ = os.Chdir(cd) }()
	}

	output, err := script.Exec(fmt.Sprintf("bazel-bin/tensorstore/tscli/tscli search -f \"file://%s\"", config.path)).Filter(
		func(r io.Reader, w io.Writer) error {
			scanner := newScanner(r)
			first := true
			for scanner.Scan() {
				if !first {
					fmt.Fprint(w, ", ")
				}
				line := scanner.Text()
				fmt.Fprint(w, line)
				first = false
			}
			fmt.Fprintln(w)
			return scanner.Err()
		}).String()
	output = fmt.Sprintf("[%s]", output)
	fmt.Println(output)
	return output, nil
	/*
		bazel-bin/tensorstore/tscli/tscli search -f "file://<mount_point>" | sed '$!s/$/,/' | sed '1s/^/[\n/'  | sed -e '$a]' > output.json
		echo "echo 3 > /proc/sys/vm/drop_caches" | sudo sh && bazel-bin/tensorstore/internal/benchmark/multi_read_benchmark --read_config=output.json

	*/
}

func newScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), math.MaxInt)
	return scanner
}

func setup() (string, error) {
	/*
			gitDep := script.Exec("apt install git")
			compilerDep := script.Exec("apt install python3.10-dev g++")
			if err = gitDep.Wait(); err != nil {
				return "", fmt.Errorf("error while installing git, stderr:%s: %w", gitDep.Error(), err)
			}
		if err = compilerDep.Wait(); err != nil {
				return "", err
			}
	*/
	wd, err := os.Getwd()
	defer func() { os.Chdir(wd) }()
	if err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp("", "tensorstore")
	if err != nil {
		return "", err
	}
	clone := script.Exec(fmt.Sprintf("git clone https://github.com/google/tensorstore.git %s/", tempDir))
	if err = clone.Wait(); err != nil {
		return "", err
	}
	os.Chdir(tempDir)
	build := script.Exec("./bazelisk.py build //tensorstore/internal/benchmark:all //tensorstore/tscli")
	if err := build.Wait(); err != nil {
		panic(err)
	}
	return tempDir, nil
}

func main() {
	checkoutDir, err := setup()
	defer func() { os.RemoveAll(checkoutDir) }()
	if err != nil {
		panic(err)
	}
	if _, err = multiReadBenchmark(checkoutDir, &multiReadConfig{
		fileIOConcurrency:   -1,
		maxInflightRequests: -1,
		numConfig:           -1,
	}); err != nil {
		panic(err)
	}

}
