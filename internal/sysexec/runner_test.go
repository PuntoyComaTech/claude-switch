package sysexec

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"testing"
)

func TestMergeEnv(t *testing.T) {
	got := mergeEnv([]string{"A=1", "B=2", "C=3"}, []string{"B=20", "C=", "D=4"})
	want := []string{"A=1", "B=20", "D=4"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOSRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	out, err := OS{}.Run(context.Background(), Cmd{Name: "sh", Args: []string{"-c", `cat; printf "$X"`}, Stdin: []byte("in-"), Env: []string{"X=env"}})
	if err != nil || string(out) != "in-env" {
		t.Fatalf("got %q, %v", out, err)
	}
	_, err = OS{}.Run(context.Background(), Cmd{Name: "sh", Args: []string{"-c", "echo boom >&2; exit 44"}})
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 44 || ee.Stderr != "boom" || ExitCode(err) != 44 {
		t.Fatalf("got %v", err)
	}
	if ExitCode(errors.New("x")) != -1 {
		t.Error("non-exit errors report -1")
	}
}
