// Command mkarchive writes a release archive (.tar.gz or .zip, chosen by the output name)
// with the same bytes on every run: fixed modification time, no owner, sorted entries.
// scripts/release.sh uses it so the build needs no tar or zip program (Git Bash has no zip).
//
//	mkarchive -o out.tar.gz -mtime 1700000000 name=path [name=path ...]
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

type entry struct {
	name string
	data []byte
	mode os.FileMode
}

func main() {
	out := flag.String("o", "", "output file, .tar.gz or .zip")
	mtime := flag.Int64("mtime", 0, "modification time of every entry, Unix seconds")
	flag.Parse()
	if *out == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: mkarchive -o out.tar.gz|out.zip [-mtime secs] name=path ...")
		os.Exit(2)
	}
	if err := run(*out, time.Unix(*mtime, 0).UTC(), flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "mkarchive:", err)
		os.Exit(1)
	}
}

func run(out string, mtime time.Time, specs []string) error {
	var entries []entry
	for _, s := range specs {
		name, path, ok := strings.Cut(s, "=")
		if !ok {
			return fmt.Errorf("%q: want name=path", s)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if !strings.HasPrefix(name, "LICENSE") {
			mode = 0o755 // the binary
		}
		entries = append(entries, entry{name, data, mode})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(out, ".zip"):
		err = writeZip(f, entries, mtime)
	case strings.HasSuffix(out, ".tar.gz"):
		err = writeTarGz(f, entries, mtime)
	default:
		err = fmt.Errorf("%s: output must end in .tar.gz or .zip", out)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func writeTarGz(w io.Writer, entries []entry, mtime time.Time) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: int64(e.mode), Size: int64(len(e.data)), ModTime: mtime, Format: tar.FormatUSTAR}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := tw.Write(e.data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func writeZip(w io.Writer, entries []entry, mtime time.Time) error {
	zw := zip.NewWriter(w)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate, Modified: mtime}
		h.SetMode(e.mode)
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := fw.Write(e.data); err != nil {
			return err
		}
	}
	return zw.Close()
}
