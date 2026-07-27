package providers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExecRunnerCapturaStdout(t *testing.T) {
	out, err := ExecRunner(context.Background(), "echo", "hola")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if strings.TrimSpace(string(out)) != "hola" {
		t.Fatalf("stdout inesperado: %q", out)
	}
}

func TestExecRunnerBinarioAusenteEsErrBinaryMissing(t *testing.T) {
	_, err := ExecRunner(context.Background(), "relai-binario-que-no-existe-jamas")
	if !errors.Is(err, ErrBinaryMissing) {
		t.Fatalf("esperaba ErrBinaryMissing, obtuve %v", err)
	}
}

func TestExecRunnerRespetaElContexto(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := ExecRunner(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("esperaba error por timeout")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("el contexto no cortó el proceso: tardó %v", elapsed)
	}
}

func TestExecRunnerIncluyeStderrEnElError(t *testing.T) {
	_, err := ExecRunner(context.Background(), "sh", "-c", "echo fallo-concreto >&2; exit 3")
	if err == nil {
		t.Fatal("esperaba error")
	}
	if !strings.Contains(err.Error(), "fallo-concreto") {
		t.Fatalf("el error debe incluir stderr, obtuve: %v", err)
	}
}
