package logging

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCreatesLogFile(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	defer l.Close()

	l.Printf("hello %s", "world")

	path := filepath.Join(dir, "android-toolbox.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected log file to exist at %s: %v", path, err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("expected log file to contain the logged message, got: %s", data)
	}
}

func TestNilLoggerMethodsAreNoops(t *testing.T) {
	var l *Logger
	l.Printf("should not panic %d", 1)
	if err := l.Close(); err != nil {
		t.Errorf("expected Close on a nil *Logger to return nil, got %v", err)
	}
}

func TestGuardRecoversPanicAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	defer l.Close()

	err = l.Guard("test-context", func() error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected Guard to convert a panic into an error")
	}
	if !strings.Contains(err.Error(), "test-context") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected the error to mention the context and panic value, got: %v", err)
	}
}

func TestGuardPassesThroughReturnedError(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	defer l.Close()

	wantErr := errors.New("boom")
	got := l.Guard("ctx", func() error { return wantErr })
	if got != wantErr {
		t.Fatalf("expected Guard to pass through the original error, got %v", got)
	}
}

func TestGuardReturnsNilWhenFnSucceeds(t *testing.T) {
	dir := t.TempDir()
	l, err := New(dir)
	if err != nil {
		t.Fatalf("New returned an error: %v", err)
	}
	defer l.Close()

	if err := l.Guard("ctx", func() error { return nil }); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
