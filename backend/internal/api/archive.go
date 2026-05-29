// backend/internal/api/archive.go
package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"path"
	"strings"

	"github.com/labstack/echo/v4"

	gftperrors "github.com/darthsoup/goblinftp/internal/errors"
	"github.com/darthsoup/goblinftp/internal/transfer"
)

// ExtractArchive extracts an uploaded archive (zip/tar/tar.gz/tar.bz2) to a remote destination.
func (h *Handler) ExtractArchive(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	destination := c.FormValue("destination")
	if destination == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "destination is required"))
	}
	fh, err := c.FormFile("archive")
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "archive file is required"))
	}
	f, err := fh.Open()
	if err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to open archive"))
	}
	defer f.Close()

	filename := strings.ToLower(fh.Filename)
	switch {
	case strings.HasSuffix(filename, ".zip"):
		data, err := io.ReadAll(f)
		if err != nil {
			return Fail(c, gftperrors.New(gftperrors.ErrInternal, "failed to read archive"))
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return Fail(c, gftperrors.New(gftperrors.ErrArchiveFormat, "invalid zip archive"))
		}
		if err := extractZip(client, zr, destination); err != nil {
			return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
		}
	case strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz"):
		gr, err := gzip.NewReader(f)
		if err != nil {
			return Fail(c, gftperrors.New(gftperrors.ErrArchiveFormat, "invalid gzip archive"))
		}
		defer gr.Close()
		if err := extractTar(client, tar.NewReader(gr), destination); err != nil {
			return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
		}
	case strings.HasSuffix(filename, ".tar.bz2"):
		if err := extractTar(client, tar.NewReader(bzip2.NewReader(f)), destination); err != nil {
			return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
		}
	case strings.HasSuffix(filename, ".tar"):
		if err := extractTar(client, tar.NewReader(f), destination); err != nil {
			return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
		}
	default:
		return Fail(c, gftperrors.New(gftperrors.ErrArchiveFormat, "unsupported archive format"))
	}
	return OK(c, nil)
}

func extractZip(client transfer.Client, zr *zip.Reader, destination string) error {
	for _, entry := range zr.File {
		outPath := path.Join(destination, entry.Name)
		if entry.FileInfo().IsDir() {
			_ = client.MakeDir(outPath)
			continue
		}
		_ = client.MakeDir(path.Dir(outPath))
		rc, err := entry.Open()
		if err != nil {
			return err
		}
		err = client.Upload(outPath, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTar(client transfer.Client, tr *tar.Reader, destination string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		outPath := path.Join(destination, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = client.MakeDir(outPath)
		case tar.TypeReg:
			_ = client.MakeDir(path.Dir(outPath))
			if err := client.Upload(outPath, tr); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateZip downloads the given remote paths and uploads a new zip to destination.
func (h *Handler) CreateZip(c echo.Context) error {
	client, ok := clientFromContext(c)
	if !ok {
		return Fail(c, gftperrors.New(gftperrors.ErrSessionNotFound, "no active connection"))
	}
	var req struct {
		Paths       []string `json:"paths"`
		Destination string   `json:"destination"`
	}
	if err := c.Bind(&req); err != nil || len(req.Paths) == 0 || req.Destination == "" {
		return Fail(c, gftperrors.New(gftperrors.ErrBadRequest, "paths and destination are required"))
	}

	pr, pw := io.Pipe()
	zw := zip.NewWriter(pw)
	errCh := make(chan error, 1)

	go func() {
		for _, p := range req.Paths {
			if err := addToZip(zw, client, p, ""); err != nil {
				pw.CloseWithError(err)
				errCh <- err
				return
			}
		}
		zw.Close()
		pw.Close()
		errCh <- nil
	}()

	if err := client.Upload(req.Destination, pr); err != nil {
		<-errCh
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	if err := <-errCh; err != nil {
		return Fail(c, gftperrors.New(gftperrors.ErrOperationFailed, err.Error()))
	}
	return OK(c, nil)
}

// addToZip recursively adds a file or directory to the zip writer.
func addToZip(zw *zip.Writer, client transfer.Client, remotePath, base string) error {
	fi, err := client.Stat(remotePath)
	if err != nil {
		return err
	}
	entryName := base + fi.Name
	if fi.IsDir {
		entryName += "/"
		entries, err := client.List(remotePath)
		if err != nil {
			return err
		}
		for _, e := range entries {
			childPath := remotePath + "/" + e.Name
			if err := addToZip(zw, client, childPath, entryName); err != nil {
				return err
			}
		}
		return nil
	}
	w, err := zw.Create(entryName)
	if err != nil {
		return err
	}
	r, err := client.Download(remotePath)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(w, r)
	return err
}
