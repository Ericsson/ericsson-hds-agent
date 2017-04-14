package agent

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

func getFilenameFromURL(url string) (string, error) {
	urlparts := strings.Split(url, "/")
	if len(urlparts) < 2 {
		return "", fmt.Errorf("could not determine filename from URL")
	}
	return urlparts[len(urlparts)-1], nil
}

func isGz(file *os.File) bool {
	return strings.HasSuffix(file.Name(), ".gz")
}

func isTar(file *os.File) bool {
	return strings.HasSuffix(file.Name(), ".tar")
}

func isTgz(file *os.File) bool {
	return strings.HasSuffix(file.Name(), ".tgz")
}

func isTarGz(file *os.File) bool {
	return strings.HasSuffix(file.Name(), ".tar.gz")
}

func unzip(gzfile *os.File) error {
	// make sure we start at the beginning of the file
	gzfile.Seek(0, 0)

	// write the files to the same directory as the original file
	dirname := filepath.Dir(gzfile.Name())

	log.Infof("unzipping %s", gzfile.Name())
	gzipReader, err := gzip.NewReader(gzfile)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	filename := filepath.Join(dirname, gzipReader.Header.Name)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	if _, err = io.Copy(file, gzipReader); err != nil {
		return err
	}

	return nil
}

func untar(file *os.File) error {
	// make sure we start at the beginning of the file
	file.Seek(0, 0)

	// write the files to the same directory as the original file
	dirname := filepath.Dir(file.Name())

	var fileReader io.ReadCloser = file
	var err error

	if isTarGz(file) || isTgz(file) {
		log.Infof("unzipping %s", file.Name())
		fileReader, err = gzip.NewReader(file)
		if err != nil {
			return err
		}
	}
	defer fileReader.Close()

	tarReader := tar.NewReader(fileReader)
	log.Infof("untarring %s", file.Name())
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		filename := filepath.Join(dirname, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			log.Infof("creating directory %s", filename)
			err = os.MkdirAll(filename, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			log.Infof("creating file %s", filename)
			newfile, err := os.Create(filename)
			if err != nil {
				return err
			}

			io.Copy(newfile, tarReader)

			err = os.Chmod(filename, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			newfile.Close()
		case tar.TypeSymlink:
			log.Infof("creating symlink %s -> %s", filename, header.Linkname)
			err = os.Symlink(header.Linkname, filename)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unable to process type [%c] in file [%s]", header.Typeflag, filename)
		}
	}
}

func contains(list []string, item string) bool {
	for _, val := range list {
		if item == val {
			return true
		}
	}
	return false
}
