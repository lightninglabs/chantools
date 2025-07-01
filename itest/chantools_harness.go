package itest

import (
	"bufio"
	"bytes"
	"errors"
	"io"
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

	return &ChantoolsProcess{
		cmd:          cmd,
		stdin:        stdin,
		stdout:       stdoutFile,
		stdoutReader: bufio.NewReader(stdoutFile),
		stderr:       bufio.NewReader(stderrPipe),
	}
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

	log.Debugf("[CHANTOOLS]: %s", resp)

	return string(resp)
}

// ReadOutputUntil reads from stdout until the given substring is found or
// timeout.
func (p *ChantoolsProcess) ReadOutputUntil(t *testing.T, substr string,
	timeout time.Duration) string {

	t.Helper()

	var out bytes.Buffer
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for chantools output")
		}
		line, err := p.stdoutReader.ReadString('\n')
		out.WriteString(line)

		log.Debugf("[CHANTOOLS]: %s", line)

		if strings.Contains(out.String(), substr) {
			return out.String()
		}

		require.NoError(t, err)
	}
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

	log.Debugf("[CHANTOOLS]: %s", out.String())
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
