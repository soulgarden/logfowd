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
	file    *os.File
	reader  *bufio.Reader
	lines   chan *entity.Line
	CurLine int64
	Offset  int64
	Path    string
	Meta    *entity.Meta
}

func NewFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(f)

	return &File{
		file:   f,
		reader: reader,
		lines:  make(chan *entity.Line, LinesChanLen),
		Path:   path,
		Meta:   &entity.Meta{},
	}, err
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

func (s *File) RestorePosition() error {
	f, err := os.Open(s.Path)
	if err != nil {
		return err
	}

	_, err = f.Seek(s.Offset, io.SeekStart)
	if err != nil {
		return err
	}

	s.reader = bufio.NewReader(f)
	s.reader.Reset(f)
	s.file = f
	s.lines = make(chan *entity.Line, LinesChanLen)

	return nil
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

	s.CurLine++
	s.Offset = offset - int64(s.reader.Buffered())

	s.lines <- &entity.Line{
		Num:  s.CurLine,
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
