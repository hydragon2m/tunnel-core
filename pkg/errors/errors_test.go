package errors

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(ErrCodeConnectionFailed, "connection failed")

	if err.Code != ErrCodeConnectionFailed {
		t.Errorf("Expected code %s, got %s", ErrCodeConnectionFailed, err.Code)
	}

	if err.Message != "connection failed" {
		t.Errorf("Expected message 'connection failed', got '%s'", err.Message)
	}

	if err.Cause != nil {
		t.Error("Expected nil cause")
	}

	if err.Stack == "" {
		t.Error("Expected stack trace to be captured")
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := Wrap(originalErr, ErrCodeInternal, "wrapped error")

	if err.Code != ErrCodeInternal {
		t.Errorf("Expected code %s, got %s", ErrCodeInternal, err.Code)
	}

	if err.Cause != originalErr {
		t.Error("Expected cause to be original error")
	}

	if !errors.Is(err, originalErr) {
		t.Error("Expected errors.Is to work with wrapped error")
	}
}

func TestWrapNil(t *testing.T) {
	err := Wrap(nil, ErrCodeInternal, "should be nil")

	if err != nil {
		t.Error("Expected nil when wrapping nil error")
	}
}

func TestWithContext(t *testing.T) {
	err := New(ErrCodeStreamNotFound, "stream not found").
		WithContext("streamID", uint32(123)).
		WithContext("agentID", "agent-1")

	if err.Context["streamID"] != uint32(123) {
		t.Error("Expected streamID in context")
	}

	if err.Context["agentID"] != "agent-1" {
		t.Error("Expected agentID in context")
	}
}

func TestWithContextMap(t *testing.T) {
	ctx := map[string]interface{}{
		"streamID": uint32(123),
		"agentID":  "agent-1",
	}

	err := New(ErrCodeStreamNotFound, "stream not found").
		WithContextMap(ctx)

	if err.Context["streamID"] != uint32(123) {
		t.Error("Expected streamID in context")
	}

	if err.Context["agentID"] != "agent-1" {
		t.Error("Expected agentID in context")
	}
}

func TestIsCode(t *testing.T) {
	err := New(ErrCodeConnectionFailed, "connection failed")

	if !IsCode(err, ErrCodeConnectionFailed) {
		t.Error("Expected IsCode to return true")
	}

	if IsCode(err, ErrCodeStreamNotFound) {
		t.Error("Expected IsCode to return false for different code")
	}

	regularErr := errors.New("regular error")
	if IsCode(regularErr, ErrCodeConnectionFailed) {
		t.Error("Expected IsCode to return false for regular error")
	}
}

func TestGetCode(t *testing.T) {
	err := New(ErrCodeConnectionFailed, "connection failed")

	code := GetCode(err)
	if code != ErrCodeConnectionFailed {
		t.Errorf("Expected code %s, got %s", ErrCodeConnectionFailed, code)
	}

	regularErr := errors.New("regular error")
	code = GetCode(regularErr)
	if code != "" {
		t.Errorf("Expected empty code for regular error, got %s", code)
	}
}

func TestErrorString(t *testing.T) {
	// Error without cause
	err1 := New(ErrCodeConnectionFailed, "connection failed")
	expected1 := "[ERR_CONNECTION_FAILED] connection failed"
	if err1.Error() != expected1 {
		t.Errorf("Expected '%s', got '%s'", expected1, err1.Error())
	}

	// Error with cause
	originalErr := errors.New("original error")
	err2 := Wrap(originalErr, ErrCodeInternal, "wrapped error")
	expected2 := "[ERR_INTERNAL] wrapped error: original error"
	if err2.Error() != expected2 {
		t.Errorf("Expected '%s', got '%s'", expected2, err2.Error())
	}
}
