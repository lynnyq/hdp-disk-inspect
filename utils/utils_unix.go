// +build !windows

package utils

import (
	"bytes"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/net/context"
)

type ShellResult struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	Err       error
	StartTime time.Time
	EndTime   time.Time
}

func (r *ShellResult) DurationMs() int64 {
	return r.EndTime.Sub(r.StartTime).Milliseconds()
}

// 执行shell命令，可设置执行超时时间
func ExecShell(ctx context.Context, command string) (*ShellResult, error) {
	result := &ShellResult{
		StartTime: time.Now(),
	}

	cmd := exec.Command("/bin/bash", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	resultChan := make(chan error, 1)
	go func() {
		resultChan <- cmd.Run()
	}()

	select {
	case <-ctx.Done():
		if cmd.Process.Pid > 0 {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		result.EndTime = time.Now()
		result.Err = errors.New("timeout killed")
		return result, result.Err
	case err := <-resultChan:
		result.EndTime = time.Now()
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					result.ExitCode = status.ExitStatus()
				}
			}
		} else {
			if cmd.ProcessState != nil {
				result.ExitCode = cmd.ProcessState.ExitCode()
			}
		}

		result.Err = err
		return result, err
	}
}
