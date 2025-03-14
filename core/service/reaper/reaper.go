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

package reaper

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reaper-build/setting"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"reaper-build/config"
	"reaper-build/core/service/meta"
	"reaper-build/tool/log"
)

const (
	// ReadmeScriptFile ...
	ReadmeScriptFile = "readme_script.sh"
	// ReadmeFile ...
	ReadmeFile = "/tmp/README"
)

// Reaper ...
type Reaper struct {
	Ctx             *meta.Context
	StartTime       time.Time
	ActiveWorkspace string
	dogFeed         bool
}

func NewReaper() (*Reaper, error) {
	context, err := ioutil.ReadFile(config.JobConfigFile())
	if err != nil {
		return nil, fmt.Errorf("read job config file error: %v", err)
	}
	var ctx *meta.Context
	if err := yaml.Unmarshal(context, &ctx); err != nil {
		return nil, fmt.Errorf("cannot unmarshal job data: %v", err)
	}
	// 初始化容器Envs, 格式为: "key=value".
	// ctx.Envs = os.Environ()
	// 初始化容器Path
	ctx.Paths = config.Path()

	reaper := &Reaper{
		Ctx: ctx,
	}
	return reaper, nil
}

func (r *Reaper) EnsureActiveWorkspace(workspace string) error {
	if workspace == "" {
		tempWorkspace, err := ioutil.TempDir(os.TempDir(), "reaper")
		if err != nil {
			return fmt.Errorf("create workspace error: %v", err)
		}
		r.ActiveWorkspace = tempWorkspace
		return os.Chdir(r.ActiveWorkspace)
	}
	err := os.MkdirAll(workspace, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}
	r.ActiveWorkspace = workspace
	return os.Chdir(r.ActiveWorkspace)
}

// BeforeExec ...
func (r *Reaper) BeforeExec() error {
	workspace := r.Ctx.Workspace

	if r.Ctx.ClassicBuild {
		workspace = r.Ctx.Workspace
	}

	r.StartTime = time.Now()

	if err := os.RemoveAll(workspace); err != nil {
		log.Warning(err.Error())
	}

	if err := r.EnsureActiveWorkspace(workspace); err != nil {
		return err
	}

	log.Info("wait for docker daemon to start ...")
	for i := 0; i < 15; i++ {
		if err := dockerInfo().Run(); err == nil {
			break
		}
		time.Sleep(time.Second * 1)
	}

	// 检查是否需要登录docker registry
	if r.Ctx.DockerRegistry != nil {
		if r.Ctx.DockerRegistry.UserName != "" {
			log.Infof("login docker registry %s", r.Ctx.DockerRegistry.Host)
			cmd := dockerLogin(r.Ctx.DockerRegistry.UserName, r.Ctx.DockerRegistry.Password, r.Ctx.DockerRegistry.Host)
			out := bytes.NewBufferString("")
			cmd.Stderr = out
			if err := cmd.Run(); err != nil {
				return errors.New(out.String())
			}
		}
	}
	// CleanWorkspace=True 意思是不使用缓存，ResetCache=True 意思是当次工作流不使用缓存
	// 如果 CleanWorkspace=True，永远不使用缓存
	// 如果 CleanWorkspace=False，本次工作流 ResetCache=False，使用缓存；本次工作流 ResetCache=True，不使用缓存
	// TODO: CleanWorkspace 和 ResetCache 严重词不达意，需要改成更合理的值
	if !r.Ctx.CleanWorkspace && !r.Ctx.ResetCache {
		// 恢复缓存
		//if _, err := os.Stat(r.GetCacheFile()); err == nil {
		// 解压缓存
		log.Info("extracting workspace ...")
		log.Info("succeed to extract workspace")
		//}
	}

	// 创建SSH目录
	if err := os.MkdirAll(path.Join(os.Getenv("HOME"), "/.ssh"), os.ModePerm); err != nil {
		return fmt.Errorf("create ssh folder error: %v", err)
	}

	// 创建发布目录
	if r.Ctx.Archive != nil && len(r.Ctx.Archive.Dir) > 0 {
		if err := os.MkdirAll(r.Ctx.Archive.Dir, os.ModePerm); err != nil {
			return fmt.Errorf("create DistDir error: %v", err)
		}
	}

	// 检查是否需要配置Gitub/Gitlab
	if r.Ctx.Git != nil {
		if err := r.Ctx.Git.WriteGithubSSHFile(); err != nil {
			return fmt.Errorf("write github ssh file error: %v", err)
		}

		if err := r.Ctx.Git.WriteGitlabSSHFile(); err != nil {
			return fmt.Errorf("write gitlab ssh file error: %v", err)
		}

		if err := r.Ctx.Git.WriteKnownHostFile(); err != nil {
			return fmt.Errorf("write known_host file error: %v", err)
		}

		if err := r.Ctx.Git.WriteSSHConfigFile(r.Ctx.Proxy); err != nil {
			return fmt.Errorf("write ssh config error: %v", err)
		}
	}

	// 清理测试目录
	if r.Ctx.GinkgoTest != nil && len(r.Ctx.GinkgoTest.ResultPath) > 0 {
		r.Ctx.GinkgoTest.ResultPath = filepath.Join(r.ActiveWorkspace, r.Ctx.GinkgoTest.ResultPath)
		log.Infof("clean test result path %s", r.Ctx.GinkgoTest.ResultPath)
		if err := os.RemoveAll(r.Ctx.GinkgoTest.ResultPath); err != nil {
			log.Warning(err.Error())
		}
		// 创建测试目录
		if err := os.MkdirAll(r.Ctx.GinkgoTest.ResultPath, os.ModePerm); err != nil {
			return fmt.Errorf("create test result path error: %v", err)
		}
	}

	return nil
}

func dockerBuildCmd(dockerfile, fullImage, ctx, buildArgs string, ignoreCache bool) *exec.Cmd {
	args := []string{"build", "--rm=true"}
	if ignoreCache {
		args = append(args, "--no-cache")
	}

	if buildArgs != "" {
		for _, val := range strings.Fields(buildArgs) {
			if val != "" {
				args = append(args, val)
			}
		}

	}
	args = append(args, []string{"-t", fullImage, "-f", dockerfile, ctx}...)
	return exec.Command(dockerExe, args...)
}

func (r *Reaper) setProxy(ctx *meta.DockerBuildCtx, cfg *meta.Proxy) {
	if cfg.EnableRepoProxy && cfg.Type == "http" {
		if !strings.Contains(strings.ToLower(ctx.BuildArgs), "--build-arg http_proxy=") {
			ctx.BuildArgs = fmt.Sprintf("%s --build-arg http_proxy=%s", ctx.BuildArgs, cfg.GetProxyURL())
		}
		if !strings.Contains(strings.ToLower(ctx.BuildArgs), "--build-arg https_proxy=") {
			ctx.BuildArgs = fmt.Sprintf("%s --build-arg https_proxy=%s", ctx.BuildArgs, cfg.GetProxyURL())
		}
	}
}

func (r *Reaper) dockerCommands() []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)
	cmds = append(
		cmds,
		dockerBuildCmd(
			r.Ctx.DockerBuildCtx.GetDockerFile(),
			r.Ctx.DockerBuildCtx.ImageName,
			r.Ctx.DockerBuildCtx.WorkDir,
			r.Ctx.DockerBuildCtx.BuildArgs,
			r.Ctx.IgnoreCache,
		),
		dockerPush(r.Ctx.DockerBuildCtx.ImageName),
	)
	return cmds
}

func (r *Reaper) runDockerBuild() error {
	if r.Ctx.DockerBuildCtx != nil {
		if r.Ctx.Proxy != nil {
			r.setProxy(r.Ctx.DockerBuildCtx, r.Ctx.Proxy)
		}

		envs := r.getUserEnvs()
		for _, c := range r.dockerCommands() {
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Dir = r.ActiveWorkspace
			c.Env = envs
			if err := c.Run(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Exec ...
func (r *Reaper) Exec() error {

	// 运行安装脚本
	if err := r.runIntallationScripts(); err != nil {
		return err
	}

	// 运行Git命令
	if err := r.runGitCmds(); err != nil {
		return err
	}

	// 生成Git commits信息
	if err := r.createReadme(ReadmeFile); err != nil {
		log.Warningf("create readme file error: %v", err)
	}

	// 运行用户脚本
	if err := r.runScripts(); err != nil {
		return err
	}

	return r.runDockerBuild()
}

// AfterExec ...
func (r *Reaper) AfterExec(upStreamErr error) error {
	var err error
	if r.Ctx.GinkgoTest != nil && r.Ctx.GinkgoTest.ResultPath != "" {
		resultPath := r.Ctx.GinkgoTest.ResultPath
		if !strings.HasPrefix(resultPath, "/") {
			resultPath = filepath.Join(r.ActiveWorkspace, resultPath)
		}
		if r.Ctx.TestType == "" {
			r.Ctx.TestType = setting.FunctionTest
		}
		if r.Ctx.TestType == setting.FunctionTest {
			log.Info("merging test result")
			if err = mergeGinkgoTestResults(
				r.Ctx.Archive.File,
				resultPath,
				r.Ctx.Archive.Dir,
				r.StartTime,
			); err != nil {
				log.Errorf("function err %v", err)
				return err
			}
		} else if r.Ctx.TestType == setting.PerformanceTest {
			log.Info("performance test result")
			if err = JmeterTestResults(
				r.Ctx.Archive.File,
				resultPath,
				r.Ctx.Archive.Dir,
			); err != nil {
				log.Errorf("performance err %v", err)
				return err
			}
		}
		//处理artifact
		if len(r.Ctx.GinkgoTest.ArtifactPaths) > 0 {
			if err = artifactsUpload(r.Ctx, r.ActiveWorkspace); err != nil {
				log.Errorf("artifactsUpload err %v", err)
				return err
			}
		}
		//if err = r.archiveTestFiles(); err != nil {
		//	log.Errorf("archiveFiles err %v", err)
		//	return err
		//}
	}

	// should archive file first, since compress cache will clean the workspace
	if upStreamErr == nil {
		if err = r.archiveTencentFiles(); err != nil {
			log.Errorf("archiveFiles err %v", err)
			return err
		}
		// 运行构建后置脚本
		if err = r.RunPostScripts(); err != nil {
			log.Errorf("RunPostScripts err %v", err)
			return err
		}

		// end here
		r.dogFeed = true
		log.Infof("build end. duration: %.2f seconds", time.Since(r.StartTime).Seconds())
	}

	return err
}

func (r *Reaper) DogFeed() bool {
	return r.dogFeed
}

func (r *Reaper) maskSecret(secrets []string, message string) string {
	out := message

	for _, val := range secrets {
		if len(val) == 0 {
			continue
		}
		out = strings.Replace(out, val, "********", -1)
	}
	return out
}

const (
	secretEnvMask = "********"
)

func (r *Reaper) maskSecretEnvs(message string) string {
	out := message

	for _, val := range r.Ctx.SecretEnvs {
		if len(val) == 0 {
			continue
		}
		sl := strings.Split(val, "=")

		if len(sl) != 2 {
			continue
		}

		if len(sl[0]) == 0 || len(sl[1]) == 0 {
			// invalid key value pair received
			continue
		}
		out = strings.Replace(out, strings.Join(sl[1:], "="), secretEnvMask, -1)
	}
	return out
}

func (r *Reaper) getUserEnvs() []string {
	envs := []string{
		"CI=true",
		"ZADIG=true",
		fmt.Sprintf("HOME=%s", config.Home()),
		fmt.Sprintf("WORKSPACE=%s", r.ActiveWorkspace),
		// TODO: readme文件可以使用别的方式代替
		fmt.Sprintf("README=%s", ReadmeFile),
	}

	r.Ctx.Paths = strings.Replace(r.Ctx.Paths, "$HOME", config.Home(), -1)
	osPath := os.Getenv("PATH")
	envs = append(envs, fmt.Sprintf("PATH=%s%s", osPath, r.Ctx.Paths))
	envs = append(envs, fmt.Sprintf("DOCKER_HOST=%s", config.DockerHost()))
	envs = append(envs, r.Ctx.Envs...)
	envs = append(envs, r.Ctx.SecretEnvs...)

	return envs
}
