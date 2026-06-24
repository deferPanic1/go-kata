package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReadNDJSON_NormalLines(t *testing.T) {
	input := `{"level":"info"}
{"level":"warn"}
{"level":"error"}
`
	var lines []string
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		`{"level":"info"}`,
		`{"level":"warn"}`,
		`{"level":"error"}`,
	}

	if len(lines) != len(expected) {
		t.Fatalf("got %d lines, want %d", len(lines), len(expected))
	}

	for i := range lines {
		if lines[i] != expected[i] {
			t.Errorf("line %d: got %q, want %q", i, lines[i], expected[i])
		}
	}
}

func TestReadNDJSON_NoTrailingNewline(t *testing.T) {
	// Последняя строка без \n
	input := `{"level":"info"}
{"level":"warn"}`

	var lines []string
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		`{"level":"info"}`,
		`{"level":"warn"}`,
	}

	if len(lines) != len(expected) {
		t.Fatalf("got %d lines, want %d", len(lines), len(expected))
	}

	for i := range lines {
		if lines[i] != expected[i] {
			t.Errorf("line %d: got %q, want %q", i, lines[i], expected[i])
		}
	}
}

func TestReadNDJSON_LongLine(t *testing.T) {
	// Строка длиннее 64KB (размер буфера)
	payload := strings.Repeat("x", 70*1024) // 70KB
	longLine := `{"data":"` + payload + `"}`
	input := `{"level":"info"}
` + longLine + `
{"level":"error"}
`
	var lines []string
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	if lines[0] != `{"level":"info"}` {
		t.Errorf("line 0: got %q, want %q", lines[0], `{"level":"info"}`)
	}
	if lines[1] != longLine {
		t.Errorf("line 1 length: got %d, want %d", len(lines[1]), len(longLine))
	}
	if lines[2] != `{"level":"error"}` {
		t.Errorf("line 2: got %q, want %q", lines[2], `{"level":"error"}`)
	}
}

func TestReadNDJSON_VeryLongLine(t *testing.T) {
	// Строка сильно больше буфера (несколько чанков ErrBufferFull)
	payload := strings.Repeat("y", 200*1024) // 200KB
	longLine := `{"data":"` + payload + `"}`
	input := longLine + "\n"

	var lines []string
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	if lines[0] != longLine {
		t.Errorf("got length %d, want %d", len(lines[0]), len(longLine))
	}
}

func TestReadNDJSON_HandleError(t *testing.T) {
	input := `{"level":"info"}
{"level":"warn"}
{"level":"error"}
`
	expectedErr := errors.New("stop on warn")

	lineCount := 0
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		lineCount++
		if string(line) == `{"level":"warn"}` {
			return expectedErr
		}
		return nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Проверяем, что ошибка завернута с номером строки
	if !errors.Is(err, expectedErr) {
		t.Errorf("error should wrap expectedErr: got %v", err)
	}

	// Проверяем, что остановились на второй строке
	if lineCount != 2 {
		t.Errorf("should process 2 lines before error, got %d", lineCount)
	}
}

func TestReadNDJSON_HandleErrorLineNumber(t *testing.T) {
	input := "line1\nline2\nline3\n"

	expectedErr := errors.New("boom")
	var caughtErr error

	ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		if string(line) == "line3" {
			return expectedErr
		}
		return nil
	})

	// Чуть позже исправим — пока ловим ошибку из возврата
	_ = caughtErr
	_ = expectedErr

	// Правильная версия:
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		if string(line) == "line3" {
			return expectedErr
		}
		return nil
	})

	if err == nil {
		t.Fatal("expected error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "line 3") && !strings.Contains(errMsg, "line3") {
		t.Errorf("error should contain line number context: got %q", errMsg)
	}
}

func TestReadNDJSON_ContextCancellation(t *testing.T) {
	// Создаём бесконечный reader
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())

	// Пишем данные в фоне
	go func() {
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(pw, `{"line":%d}`+"\n", i)
		}
	}()

	processed := 0
	errCh := make(chan error, 1)

	go func() {
		errCh <- ReadNDJSON(ctx, pr, func(line []byte) error {
			processed++
			if processed == 10 {
				cancel() // отменяем после 10 строк
			}
			// Небольшая задержка, чтобы гарантировать гонку
			time.Sleep(time.Millisecond)
			return nil
		})
	}()

	var err error
	select {
	case err = <-errCh:
	case <-time.After(time.Second):
		t.Fatal("timeout: context cancellation didn't stop reading")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestReadNDJSON_EmptyInput(t *testing.T) {
	input := ""

	var lines []string
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestReadNDJSON_OnlyNewlines(t *testing.T) {
	input := "\n\n\n"

	var lines []string
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Пустые строки после удаления \n — это пустые строки
	if len(lines) != 3 {
		t.Errorf("expected 3 empty lines, got %d", len(lines))
	}

	for i, line := range lines {
		if line != "" {
			t.Errorf("line %d: expected empty, got %q", i, line)
		}
	}
}

func TestReadNDJSON_MultipleLongLines(t *testing.T) {
	// Несколько длинных строк подряд — проверяем, что буфер переиспользуется
	payload := strings.Repeat("a", 100*1024) // 100KB
	lines := make([]string, 5)
	for i := range lines {
		lines[i] = fmt.Sprintf(`{"index":%d,"data":"%s"}`, i, payload)
	}

	input := ""
	for _, line := range lines {
		input += line + "\n"
	}

	var result []string
	err := ReadNDJSON(context.Background(), strings.NewReader(input), func(line []byte) error {
		result = append(result, string(line))
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != len(lines) {
		t.Fatalf("got %d lines, want %d", len(result), len(lines))
	}

	for i := range result {
		if result[i] != lines[i] {
			t.Errorf("line %d mismatch: lengths %d vs %d", i, len(result[i]), len(lines[i]))
		}
	}
}

// Бенчмарк для проверки аллокаций
func BenchmarkReadNDJSON(b *testing.B) {
	payload := strings.Repeat("x", 1000)
	input := ""
	for i := 0; i < 100; i++ {
		input += fmt.Sprintf(`{"line":%d,"data":"%s"}`, i, payload) + "\n"
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(input)
		ReadNDJSON(context.Background(), reader, func(line []byte) error {
			_ = line
			return nil
		})
	}
}
