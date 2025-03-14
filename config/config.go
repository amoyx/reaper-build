/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import "os"

func Home() string {
	if os.Getenv("HOME") != "" {
		return os.Getenv("HOME")
	}
	return "/root"
}

func PkgFile() string {
	return ""
}

func JobConfigFile() string {
	if os.Getenv("JOB_CONFIG_FILE") != "" {
		return os.Getenv("JOB_CONFIG_FILE")
	}
	return "/opt/job-config.yaml"
}

func DockerAuthDir() string {
	return ""
}

func Path() string {
	return ""
}

func DockerHost() string {
	if os.Getenv("DOCKER_HOST") != "" {
		return os.Getenv("DOCKER_HOST")
	}
	return "tcp://dind-1.dind:2375"
}

func BuildURL() string {
	return ""
}
