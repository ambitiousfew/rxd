package log

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

type tableTest struct {
	level       Level
	expectEmpty bool
	msg         string
	fields      []Field
}

// TestLogger_Error tests logging each level using LevelDebug
// and verifies the output is correct since LevelDebug is the lowest level
// and should log all everything.
func TestLogger_Debug(t *testing.T) {
	var testbuf bytes.Buffer

	loggingLevel := LevelDebug

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, loggingLevel, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, false, msg, fields},
		{LevelInfo, false, msg, fields},
		{LevelNotice, false, msg, fields},
		{LevelWarning, false, msg, fields},
		{LevelError, false, msg, fields},
		{LevelCritical, false, msg, fields},
		{LevelAlert, false, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

func TestLogger_Info(t *testing.T) {
	var testbuf bytes.Buffer
	loggingLevel := LevelInfo

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, loggingLevel, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, true, msg, fields},
		{LevelInfo, false, msg, fields},
		{LevelNotice, false, msg, fields},
		{LevelWarning, false, msg, fields},
		{LevelError, false, msg, fields},
		{LevelCritical, false, msg, fields},
		{LevelAlert, false, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

func TestLogger_Notice(t *testing.T) {
	var testbuf bytes.Buffer

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, LevelNotice, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, true, msg, fields},
		{LevelInfo, true, msg, fields},
		{LevelNotice, false, msg, fields},
		{LevelWarning, false, msg, fields},
		{LevelError, false, msg, fields},
		{LevelCritical, false, msg, fields},
		{LevelAlert, false, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

func TestLogger_Warning(t *testing.T) {
	var testbuf bytes.Buffer

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, LevelWarning, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, true, msg, fields},
		{LevelInfo, true, msg, fields},
		{LevelNotice, true, msg, fields},
		{LevelWarning, false, msg, fields},
		{LevelError, false, msg, fields},
		{LevelCritical, false, msg, fields},
		{LevelAlert, false, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

func TestLogger_Error(t *testing.T) {
	var testbuf bytes.Buffer

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, LevelError, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, true, msg, fields},
		{LevelInfo, true, msg, fields},
		{LevelNotice, true, msg, fields},
		{LevelWarning, true, msg, fields},
		{LevelError, false, msg, fields},
		{LevelCritical, false, msg, fields},
		{LevelAlert, false, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

func TestLogger_Critical(t *testing.T) {
	var testbuf bytes.Buffer

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, LevelCritical, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, true, msg, fields},
		{LevelInfo, true, msg, fields},
		{LevelNotice, true, msg, fields},
		{LevelWarning, true, msg, fields},
		{LevelError, true, msg, fields},
		{LevelCritical, false, msg, fields},
		{LevelAlert, false, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

func TestLogger_Alert(t *testing.T) {
	var testbuf bytes.Buffer

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, LevelAlert, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, true, msg, fields},
		{LevelInfo, true, msg, fields},
		{LevelNotice, true, msg, fields},
		{LevelWarning, true, msg, fields},
		{LevelError, true, msg, fields},
		{LevelCritical, true, msg, fields},
		{LevelAlert, false, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

func TestLogger_Emergency(t *testing.T) {
	var testbuf bytes.Buffer

	msg := "test message"
	fields := []Field{
		String("test_key", "test_value"),
	}

	runTests(t, &testbuf, LevelEmergency, []struct {
		level       Level
		expectEmpty bool
		msg         string
		fields      []Field
	}{
		{LevelDebug, true, msg, fields},
		{LevelInfo, true, msg, fields},
		{LevelNotice, true, msg, fields},
		{LevelWarning, true, msg, fields},
		{LevelError, true, msg, fields},
		{LevelCritical, true, msg, fields},
		{LevelAlert, true, msg, fields},
		{LevelEmergency, false, msg, fields},
	})
}

// runTests is a utility function to run the tests for each level
// and verify the output is correct.
func runTests(t *testing.T, buf *bytes.Buffer, loggingLevel Level, tests []struct {
	level       Level
	expectEmpty bool
	msg         string
	fields      []Field
}) {

	if buf == nil {
		t.Fatalf("expected test buffer to be created, got nil")
	}

	handler := &bufferHandler{
		buf: buf,
	}

	// create a new logger with a test buffer handler
	logger := NewLogger(loggingLevel, handler)
	if logger == nil {
		t.Fatalf("expected logger to be created, got nil")
	}

	for _, test := range tests {
		level := test.level
		logger.Log(level, test.msg, test.fields...)
		var expect string
		if !test.expectEmpty {
			expect = fmt.Sprintf("%s %s %s=%s \n", level.String(), test.msg, test.fields[0].Key, test.fields[0].Value)
		}

		if got := buf.String(); got != expect {
			t.Errorf("expected %q, got %q", expect, got)
		}
		buf.Reset()
	}
}

type bufferHandler struct {
	buf io.Writer
}

func (h *bufferHandler) Handle(level Level, message string, fields []Field) {
	var b strings.Builder
	b.WriteString(level.String())
	b.WriteString(" ")
	b.WriteString(message)
	b.WriteString(" ")
	for _, field := range fields {
		b.WriteString(field.Key + "=" + field.Value + " ")
	}
	b.WriteString("\n")
	h.buf.Write([]byte(b.String()))
	b.Reset()
}
