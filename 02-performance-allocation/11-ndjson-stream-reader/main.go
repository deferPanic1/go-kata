package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

func ReadNDJSON(ctx context.Context, r io.Reader, handle func([]byte) error) error {
	line := []byte{}
	br := bufio.NewReaderSize(r, 64*1024)
	lineCounter := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		chunk, err := br.ReadSlice('\n')

		line = append(line, chunk...)

		if err == bufio.ErrBufferFull {
			continue
		}

		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		if err == io.EOF {
			if len(line) > 0 {
				lineCounter++
				if hErr := handle(line); hErr != nil {
					return fmt.Errorf("line %d: %w", lineCounter, hErr)
				}
			}
			return nil
		}

		if err != nil {
			return fmt.Errorf("line %d: %w", lineCounter+1, err)
		}

		lineCounter++
		if hErr := handle(line); hErr != nil {
			return fmt.Errorf("line %d: %w", lineCounter, hErr)
		}

		line = line[:0]
	}
}

func main() {
}
