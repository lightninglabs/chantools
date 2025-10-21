package itest

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ChantoolsProcess wraps a running chantools process for integration testing.
type ChantoolsProcess struct {
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       *os.File
	stdoutReader *bufio.Reader
	stderr       *bufio.Reader
}

// StartChantools starts the chantools binary with the given arguments.
func StartChantools(t *testing.T, args ...string) *ChantoolsProcess {
	t.Helper()

	log.Debugf("Calling chantools with args: %v", args)

	args = append([]string{"--nologfile"}, args...)
	cmd := exec.Command("chantools", args...)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)

	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderrPipe, err := cmd.StderrPipe()
	require.NoError(t, err)

	stdoutFile, ok := stdoutPipe.(*os.File)
	require.True(t, ok)

	require.NoError(t, cmd.Start())

	proc := &ChantoolsProcess{
		cmd:          cmd,
		stdin:        stdin,
		stdout:       stdoutFile,
		stdoutReader: bufio.NewReader(stdoutFile),
		stderr:       bufio.NewReader(stderrPipe),
	}

	proc.AssertNoStderr(t)
	return proc
}

func (p *ChantoolsProcess) AssertNoStderr(t *testing.T) {
	t.Helper()

	require.Never(t, func() bool {
		stderrBytes, err := io.ReadAll(p.stderr)
		if isProcessExitErr(err) {
			return false
		}
		require.NoError(t, err)

		if len(stderrBytes) > 0 {
			t.Logf("Stderr has unexpected output: %s", stderrBytes)
			return true
		}

		return false
	}, shortTimeout, testTick)
}

// WriteInput writes input to the process's stdin.
func (p *ChantoolsProcess) WriteInput(t *testing.T, input string) {
	t.Helper()

	_, err := io.WriteString(p.stdin, input)
	require.NoError(t, err, "failed to write input to chantools")
}

// ReadAllOutput reads all output from the process's stdout until EOF.
func (p *ChantoolsProcess) ReadAllOutput(t *testing.T) string {
	t.Helper()

	resp, err := io.ReadAll(p.stdout)
	require.NoError(t, err, "failed to read chantools output")

	log.Debugf("[CHANTOOLS]: %s", strings.TrimSpace(string(resp)))

	return string(resp)
}

// ReadAvailableOutput reads as many bytes as possible from stdout until the
// timeout elapses.
func (p *ChantoolsProcess) ReadAvailableOutput(t *testing.T,
	timeout time.Duration) string {

	t.Helper()

	err := p.stdout.SetReadDeadline(time.Now().Add(timeout))
	require.NoError(t, err, "failed to set read deadline")

	defer func() {
		_ = p.stdout.SetReadDeadline(time.Time{})
	}()

	var out bytes.Buffer
	for {
		buf := make([]byte, 1024)
		n, err := p.stdoutReader.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			out.WriteString(chunk)
		}
		if err != nil {
			if errors.Is(err, io.EOF) ||
				errors.Is(err, os.ErrDeadlineExceeded) {

				break
			}

			time.Sleep(50 * time.Millisecond)
		}
	}

	log.Debugf("[CHANTOOLS]: %s", strings.TrimSpace(out.String()))
	return out.String()
}

// Wait waits for the process to exit.
func (p *ChantoolsProcess) Wait(t *testing.T) {
	t.Helper()

	require.NoError(t, p.cmd.Wait())
}

// Kill kills the process.
func (p *ChantoolsProcess) Kill(t *testing.T) {
	t.Helper()

	require.NoError(t, p.cmd.Process.Kill())
}

func isProcessExitErr(err error) bool {
	var pathError *fs.PathError
	if err != nil && errors.As(err, &pathError) {
		return errors.Is(pathError.Err, fs.ErrClosed)
	}

	return false
}
