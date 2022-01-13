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
	file   *os.File
	reader *bufio.Reader
	lines  chan *entity.Line
	Size   int64
	Offset int64
	Path   string
	Meta   *entity.Meta
}

func NewFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &File{
		file:   f,
		reader: bufio.NewReader(f),
		lines:  make(chan *entity.Line, LinesChanLen),
		Path:   path,
		Meta:   &entity.Meta{},
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

	s.Size = info.Size()
	s.Offset = offset

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

	f, err := os.Open(s.Path)
	if err != nil {
		return err
	}

	s.reader.Reset(f)

	s.file = f

	s.Offset = 0

	return nil
}

func (s *File) RestorePosition() error {
	f, err := os.Open(s.Path)
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

	size := s.Size // nolint: ifshort

	s.Size = info.Size()

	var offset int64

	if s.Size >= size {
		offset, err = f.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}
	}

	// file probably was truncated, reset position
	s.Offset = offset

	s.reader.Reset(s.file)

	return nil
}

func (s *File) IsTruncated() (bool, error) {
	size := s.Size

	info, err := s.file.Stat()
	if err != nil {
		return false, err
	}

	s.Size = info.Size()

	return size > 0 && size > s.Size, nil
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

	s.Offset = offset - int64(s.reader.Buffered())

	s.lines <- &entity.Line{
		Pos:  s.Offset,
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
