package streamio

import (
	"context"
	"io"
	"log"
	"time"
)

func CopyWithTimeout(w io.Writer, r io.Reader, length int64, ctx context.Context) {
	buf := make([]byte, 64*1024)
	written := int64(0)
	lastProgress := time.Now()

	type flusher interface {
		Flush()
	}

	canFlush, ok := w.(flusher)

	for written < length {
		select {
		case <-ctx.Done():
			return
		default:
		}

		maxRead := int64(len(buf))
		if length-written < maxRead {
			maxRead = length - written
		}

		n, err := r.Read(buf[:maxRead])

		if n == 0 && err == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				return
			}
			written += int64(n)
			lastProgress = time.Now()

			if ok {
				canFlush.Flush()
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			if time.Since(lastProgress) > 30*time.Second {
				log.Printf("Read timeout after 30 seconds")
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
