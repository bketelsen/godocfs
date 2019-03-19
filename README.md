# godocfs
A FUSE filesystem that presents your GOPATH as a directory, with file entries containing the `go doc` for each package.

## Building

```
go get -u github.com/bketelsen/godocfs
```

## Usage

```
mkdir $HOME/godoc
godocfs $HOME/godoc
cd $HOME/godoc/github.com/pkg/errors
ls
> godoc
cat godoc
> // package errors
> ...
```
## Disclaimer

This is a toy, you should use `go doc pkgname` instead.