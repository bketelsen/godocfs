package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "bazil.org/fuse/fs/fstestutil"
	"bazil.org/fuse/fuseutil"
	"golang.org/x/net/context"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	mountpoint := flag.Arg(0)

	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("godocfs"),
		fuse.Subtype("godocfs"),
		fuse.LocalVolume(),
		fuse.VolumeName("godocfs"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if err != nil {
		log.Fatal(err)
	}
	fsys := NewFS()
	err = fs.Serve(c, fsys)
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

type FS struct {
	root     Dir
	basePath string
}

func NewFS() FS {

	gp := os.Getenv("GOPATH")
	paths := filepath.SplitList(gp)

	return FS{basePath: paths[0], root: rootDir(paths[0])}
}

func (f FS) Root() (fs.Node, error) {
	return f.root, nil
}
func rootDir(base string) Dir {
	return Dir{
		inode: 1,
		mode:  os.ModeDir,
		name:  "GOPATH",
		path:  filepath.Join(base, "src"),
	}

}

// Dir implements both Node and Handle for the root directory.
type Dir struct {
	inode uint64
	mode  os.FileMode
	name  string
	gid   uint32
	uid   uint32
	path  string
}

func (d Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = d.inode
	a.Mode = d.mode
	a.Uid = d.uid
	a.Gid = d.gid
	return nil
}

func (d Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {

	if name == "godoc" {

		inode := (d.inode * 100)
		f := &File{
			inode:       inode,
			parentinode: d.inode,
			name:        "godoc",
			mode:        0444,
			gid:         uint64(os.Getgid()),
			uid:         uint64(os.Getuid()),
			fullPath:    filepath.Join(d.path, "godoc"),
		}
		return f, nil
	}
	files, err := ioutil.ReadDir(d.path)
	if err != nil {
		log.Fatal(err)
	}

	for i, file := range files {

		inode := (d.inode * 100) + uint64(i+1)
		if file.Name() == name {

			if file.IsDir() {
				return Dir{
					inode: inode,
					name:  name,
					path:  filepath.Join(d.path, name),
					mode:  os.ModeDir | 0555,
					uid:   uint32(os.Getuid()),
					gid:   uint32(os.Getgid()),
				}, nil

			}

		}
	}
	return nil, fuse.ENOENT
}

func (d Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var entries []fuse.Dirent
	var de fuse.Dirent

	files, err := ioutil.ReadDir(d.path)
	if err != nil {
		log.Fatal(err)
	}

	inode := (d.inode * 100)
	ded := fuse.Dirent{
		Inode: inode,
		Type:  fuse.DT_File,
		Name:  "godoc",
	}
	entries = append(entries, ded)
	for i, file := range files {

		inode := (d.inode * 100) + uint64(i+1)
		if file.IsDir() {
			if file.Name() != ".git" {
				de = fuse.Dirent{
					Inode: inode,
					Type:  fuse.DT_Dir,
					Name:  file.Name(),
				}

				entries = append(entries, de)
			}
		}
	}
	return entries, nil
}

type File struct {
	inode uint64
	//	client   *Client
	mode        os.FileMode
	location    string
	fullPath    string
	name        string
	parentinode uint64
	gid         uint64
	uid         uint64
	content     []byte
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {

	a.Inode = f.parentinode * 100
	a.Mode = 0444
	bb, err := f.ReadAll()
	if err != nil {
		panic(err)
	}
	f.content = bb
	a.Size = uint64(len(bb))
	//	a.Uid = uint32(os.Getuid())
	//	a.Gid = uint32(os.Getgid())
	//	a.Valid = 20 * time.Second
	return nil
}

func (f *File) ReadAll() ([]byte, error) {

	gp := os.Getenv("GOPATH")
	paths := filepath.SplitList(gp)
	first := filepath.ToSlash(filepath.Join(paths[0], "src"))
	pkg := strings.Replace(f.fullPath, first, "", -1)

	pkg = strings.Replace(pkg, "godoc", "", -1)
	pkg = filepath.Dir(pkg)
	if strings.HasPrefix(pkg, "/") {
		pkg = pkg[1:]
	}

	if f.name == "godoc" {
		cmd := exec.Command("go", "doc", pkg)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
		return out.Bytes(), err
	}
	return []byte{}, nil
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if !req.Flags.IsReadOnly() {
		return nil, fuse.Errno(syscall.EACCES)
	}
	resp.Flags |= fuse.OpenKeepCache
	return f, nil
}

var _ fs.Handle = (*File)(nil)

var _ fs.HandleReader = (*File)(nil)

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	t := string(f.content)
	fuseutil.HandleRead(req, resp, []byte(t))
	return nil
}
