# go-billy [![GoDoc](https://godoc.org/gopkg.in/src-d/go-billy.v2?status.svg)](https://godoc.org/gopkg.in/src-d/go-billy.v2) [![Build Status](https://travis-ci.org/src-d/go-billy.svg)](https://travis-ci.org/src-d/go-billy) [![codebeat badge](https://codebeat.co/badges/03bdec03-b477-4472-bbe3-b552e3799174)](https://codebeat.co/projects/github-com-src-d-go-billy)

An interface to abstract several storages.

This library was extracted from
[src-d/go-git](https://github.com/src-d/go-git).

## Installation

```go
go get -u gopkg.in/src-d/go-billy.v2/...
```

## Why billy?

The library billy deals with storage systems and Billy is the name of a well-known, IKEA
bookcase. That's it.

## Usage

Billy exposes filesystems using the
[`Filesystem` interface](https://godoc.org/github.com/src-d/go-billy#Filesystem).
Each filesystem implementation gives you a `New` method, whose arguments depend on
the implementation itself, that returns a new `Filesystem`.

The following example caches in memory all readable files in a directory from any
billy's filesystem implementation.

```go
func LoadToMemory(fs billy.Filesystem, path string) (*memory.Memory, error) {
	memory := memory.New()

	files, err := fs.ReadDir("/")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() {
			orig, err := fs.Open(file.Name())
			if err != nil {
				continue
			}

			dest, err := memory.Create(file.Name())
			if err != nil {
				continue
			}

			if _, err = io.Copy(dest, orig); err != nil {
				return nil, err
			}
		}
	}

	return memory, nil
}
```

## License

MIT, see [LICENSE](LICENSE)
