package seekable2

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strings"

	"gopkg.in/src-d/go-git.v3/core"
	"gopkg.in/src-d/go-git.v3/storage/memory"
	"gopkg.in/src-d/go-git.v3/storage/seekable2/internal/gitdir"
	"gopkg.in/src-d/go-git.v3/utils/fs"
)

// ObjectStorage is an implementation of core.ObjectStorage that stores
// data on disk in the standard git format (this is, the .git directory).
//
// Zero values of this type are not safe to use, see the New function below.
//
// Currently only reads are supported, no writting.
//
// Also values from this type are not yet able to track changes on disk, this is,
// Gitdir values will get outdated as soon as repositories change on disk.
type ObjectStorage struct {
	dir *gitdir.GitDir
}

// New returns a new ObjectStorage for the git directory at the specified path.
func New(fs fs.FS, path string) (*ObjectStorage, error) {
	s := &ObjectStorage{}

	var err error
	s.dir, err = gitdir.New(fs, path)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Set adds a new object to the storage. As this functionality is not
// yet supported, this method always returns a "not implemented yet"
// error an zero hash.
func (s *ObjectStorage) Set(core.Object) (core.Hash, error) {
	return core.ZeroHash, fmt.Errorf("not implemented yet")
}

// Get returns the object with the given hash, by searching for it in
// the packfile.
func (s *ObjectStorage) Get(h core.Hash) (core.Object, error) {
	fs, path, err := s.dir.Objectfile(h)
	if err != nil {
		panic(err)
		return nil, err
	}

	f, err := fs.Open(path)
	if err != nil {
		panic(err)
		return nil, err
	}

	defer func() {
		errClose := f.Close()
		if err == nil {
			err = errClose
		}
	}()

	commit, err := readObject(f)
	if err != nil {
		panic(err)
		return nil, err
	}

	return commit, err
}

func readObject(r io.Reader) (core.Object, error) {
	cont, err := readZip(r)
	if err != nil {
		return nil, err
	}

	return memory.NewObject(core.CommitObject, int64(len(cont)), cont), nil
}

func readZip(r io.Reader) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := inflate(buf, r)

	return buf.Bytes(), err
}

func inflate(w io.Writer, r io.Reader) (err error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		if err != zlib.ErrHeader {
			return fmt.Errorf("zlib reading error: %s", err)
		}
	}

	defer func() {
		closeErr := zr.Close()
		if err == nil {
			err = closeErr
		}
	}()

	_, err = io.Copy(w, zr)

	return err
}

// Iter returns an iterator for all the objects in the packfile with the
// given type.
func (s *ObjectStorage) Iter(t core.ObjectType) (core.ObjectIter, error) {
	var objects []core.Object

	// for hash := range s.index {
	// 	object, err := s.Get(hash)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	if object.Type() == t {
	// 		objects = append(objects, object)
	// 	}
	// }

	return core.NewObjectSliceIter(objects), nil
}

const (
	headErrPrefix      = "cannot get HEAD reference:"
	symrefCapapability = "symref"
	headRefPrefix      = "HEAD:"
)

// Head returns the hash of the HEAD reference
func (s *ObjectStorage) Head() (core.Hash, error) {
	cap, err := s.dir.Capabilities()
	if err != nil {
		fmt.Println("Error 0000000000000000000000000000000000000000000000000000000")
		return core.ZeroHash, fmt.Errorf("%s %s", headErrPrefix, err)
	}
	ok := cap.Supports(symrefCapapability)
	if !ok {
		return core.ZeroHash,
			fmt.Errorf("%s symref capability not supported", headErrPrefix)
	}

	symrefs := cap.Get(symrefCapapability)
	var headRef string
	for _, ref := range symrefs.Values {
		if strings.HasPrefix(ref, headRefPrefix) {
			headRef = strings.TrimPrefix(ref, headRefPrefix)
		}
	}
	fmt.Println(headRef)
	if headRef == "" {
		return core.ZeroHash, fmt.Errorf("%s HEAD reference not found",
			headErrPrefix)
	}
	refs, err := s.dir.Refs()
	fmt.Println(refs)
	if err != nil {
		return core.ZeroHash, fmt.Errorf("%s %s", headErrPrefix, err)
	}

	head, ok := refs[headRef]
	if !ok {
		return core.ZeroHash, fmt.Errorf("%s reference %q not found",
			headErrPrefix, headRef)
	}

	return head, nil
}
