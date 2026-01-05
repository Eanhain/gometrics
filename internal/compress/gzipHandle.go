package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

var gzPoolWriter = sync.Pool{
	New: func() any {
		w := gzip.NewWriter(io.Discard)
		gzip.NewWriterLevel(w, gzip.BestSpeed)
		return w
	},
}

var emptyGzip = []byte{
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff, 0x00,
	0x00, 0x00,
}

var gzPoolReader = sync.Pool{
	New: func() any {
		r, err := gzip.NewReader(bytes.NewReader(emptyGzip))
		if err != nil {
			panic(err)
		}
		return r
	},
}

func GzipHandleReader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzPoolReader.Get().(*gzip.Reader)

		defer gzPoolReader.Put(gz)

		gz.Reset(r.Body)

		defer r.Body.Close()
		r.Body = gz
		next.ServeHTTP(w, r)
	})
}

func (w gzipWriter) Write(b []byte) (int, error) {
	// w.Writer будет отвечать за gzip-сжатие, поэтому пишем в него
	return w.Writer.Write(b)
}

func GzipHandleWriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// проверяем, что клиент поддерживает gzip-сжатие
		// это упрощённый пример. В реальном приложении следует проверять все
		// значения r.Header.Values("Accept-Encoding") и разбирать строку
		// на составные части, чтобы избежать неожиданных результатов
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") &&
			!(strings.Contains(r.Header.Get("Content-Type"), "text/html") ||
				strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
				strings.Contains(r.Header.Get("Content-Type"), "application/x-gob")) {
			// если gzip не поддерживается, передаём управление
			// дальше без изменений
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")

		gz := gzPoolWriter.Get().(*gzip.Writer)
		defer gzPoolWriter.Put(gz)

		gz.Reset(w)
		defer gz.Close()

		next.ServeHTTP(&gzipWriter{ResponseWriter: w, Writer: gz}, r)

	})
}
