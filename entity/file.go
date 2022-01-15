package entity

type File struct {
	Size   int64
	Offset int64
	Path   string
	Meta   *Meta
}
