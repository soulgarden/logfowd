package file

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/soulgarden/logfowd/entity"
)

const LinesChanLen = 1024 * 10

type File struct {
	file       *os.File
	EntityFile *entity.File
	reader     *bufio.Reader
	lines      chan *entity.Line
}

func NewFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	entityFile := &entity.File{
		Size:   0,
		Offset: 0,
		Path:   path,
		Meta:   &entity.Meta{},
	}

	return &File{
		file:       f,
		reader:     bufio.NewReader(f),
		lines:      make(chan *entity.Line, LinesChanLen),
		EntityFile: entityFile,
	}, err
}

func (s *File) SeekEnd() error {
	info, err := s.file.Stat()
	if err != nil {
		return err
	}

	offset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	s.EntityFile.Size = info.Size()
	s.EntityFile.Offset = offset

	s.reader.Reset(s.file)

	return nil
}

func (s *File) Read() error {
	for {
		err := s.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}
	}
}

func (s *File) Reset() error {
	if err := s.file.Close(); err != nil {
		return err
	}

	f, err := os.Open(s.EntityFile.Path)
	if err != nil {
		return err
	}

	s.reader.Reset(f)

	s.file = f

	s.EntityFile.Offset = 0

	return nil
}

func (s *File) RestorePosition() error {
	f, err := os.Open(s.EntityFile.Path)
	if err != nil {
		return err
	}

	s.file = f
	s.reader = bufio.NewReader(s.file)
	s.lines = make(chan *entity.Line, LinesChanLen)

	info, err := s.file.Stat()
	if err != nil {
		return err
	}

	size := s.EntityFile.Size // nolint: ifshort

	s.EntityFile.Size = info.Size()

	var offset int64

	if s.EntityFile.Size >= size {
		offset, err = f.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}
	}

	// file probably was truncated, reset position
	s.EntityFile.Offset = offset

	s.reader.Reset(s.file)

	return nil
}

func (s *File) IsTruncated() (bool, error) {
	size := s.EntityFile.Size

	info, err := s.file.Stat()
	if err != nil {
		return false, err
	}

	s.EntityFile.Size = info.Size()

	return size > 0 && size > s.EntityFile.Size, nil
}

func (s *File) readLine() error {
	str, err := s.reader.ReadString('\n')
	if err != nil {
		return err
	}

	offset, err := s.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	s.EntityFile.Offset = offset - int64(s.reader.Buffered())

	s.lines <- &entity.Line{
		Pos:  s.EntityFile.Offset,
		Str:  strings.TrimRight(str, "\n"),
		Time: time.Now(),
	}

	return nil
}

func (s *File) ListenLine() <-chan *entity.Line {
	return s.lines
}

func (s *File) Close() error {
	err := s.file.Close()

	close(s.lines)

	return err
}
