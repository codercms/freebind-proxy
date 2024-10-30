package proxy

import (
	"errors"
	"io"
	"net"
	"os"
	"time"
)

// NetConnTimeoutReadWriter sets [net.Conn] read/write deadlines on read
type NetConnTimeoutReadWriter struct {
	conn    net.Conn
	timeout time.Duration
}

func (rw *NetConnTimeoutReadWriter) Read(p []byte) (n int, err error) {
	_ = rw.conn.SetReadDeadline(time.Now().Add(rw.timeout))

	return rw.conn.Read(p)
}

func (rw *NetConnTimeoutReadWriter) Write(p []byte) (n int, err error) {
	_ = rw.conn.SetWriteDeadline(time.Now().Add(rw.timeout))

	return rw.conn.Write(p)
}

var errInvalidWrite = errors.New("invalid write result")

// CopyBufferWithTimeout is a copy of [io.CopyBuffer] but with retries on timeouts
func CopyBufferWithTimeout(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	if buf == nil {
		size := 32 * 1024
		buf = make([]byte, size)
	}

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			var nw int
			var ew error

			//log.Printf("Read %d bytes: %s", nr, string(buf[:nr]))

		writeLoop:
			//oldNw := nw
			nw, ew = dst.Write(buf[nw:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errInvalidWrite
				}
			}

			//log.Printf("Written %d bytes: %s", nw, string(buf[oldNw:nr]))

			written += int64(nw)
			if ew != nil {
				// Continue on write timeout errors
				if os.IsTimeout(ew) {
					goto writeLoop
				}

				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				// Continue on read timeout errors
				if os.IsTimeout(er) {
					continue
				}

				err = er
			}
			break
		}
	}
	return written, err
}
