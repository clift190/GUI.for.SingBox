package bridge

import (
	"bufio"
	"errors"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) Exec(path string, args []string, options ExecOptions) FlagResult {
	log.Printf("Exec: %s %s %v", path, args, options)

	exe_path := GetPath(path)

	if _, err := os.Stat(exe_path); os.IsNotExist(err) {
		exe_path = path
	}

	cmd := exec.Command(exe_path, args...)
	HideExecWindow(cmd)

	for key, value := range options.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	output := ""
	if options.Convert {
		output = ConvertByte2String(out)
	} else {
		output = string(out)
	}

	return FlagResult{true, output}
}

func (a *App) ExecBackground(path string, args []string, outEvent string, endEvent string, options ExecOptions) FlagResult {
	log.Printf("ExecBackground: %s %s %v", path, args, options)

	exe_path := GetPath(path)

	if _, err := os.Stat(exe_path); os.IsNotExist(err) {
		exe_path = path
	}

	cmd := exec.Command(exe_path, args...)
	HideExecWindow(cmd)

	for key, value := range options.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	err = cmd.Start()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	if outEvent != "" {
		wg := sync.WaitGroup{}
		wg.Add(2)

		outScanner := bufio.NewScanner(stdout)
		go func() {
			defer wg.Done()
			for outScanner.Scan() {
				text := outScanner.Text()
				runtime.EventsEmit(a.Ctx, outEvent, text)
			}
		}()

		errScanner := bufio.NewScanner(stderr)
		go func() {
			defer wg.Done()
			for errScanner.Scan() {
				text := errScanner.Text()
				runtime.EventsEmit(a.Ctx, outEvent, text)
			}
		}()

		go func() {
			wg.Wait()
			if endEvent != "" {
				runtime.EventsEmit(a.Ctx, endEvent)
			}
		}()
	}

	pid := cmd.Process.Pid

	return FlagResult{true, strconv.Itoa(pid)}
}

func (a *App) ProcessInfo(pid int32) FlagResult {
	log.Printf("ProcessInfo: %d", pid)

	proc, err := process.NewProcess(pid)
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	name, err := proc.Name()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	return FlagResult{true, name}
}

func (a *App) KillProcess(pid int) FlagResult {
	log.Printf("KillProcess: %d", pid)

	process, err := os.FindProcess(pid)
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	err = KillProcessImpl(process)
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	err = waitForProcessExitWithTimeout(process, 15)
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	return FlagResult{true, "Success"}
}

func waitForProcessExitWithTimeout(process *os.Process, timeoutSeconds int64) error {
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		process.Kill()
		return errors.New("timeout waiting for process to exit")
	}

	return nil
}
