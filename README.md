# jsonfile:

Use `jsonfile` to persist a Go value to a JSON file.

```go
type JSONFile
    func Load[Data any](path string) (*JSONFile[Data], error)
    func New[Data any](path string) (*JSONFile[Data], error)
    func (p *JSONFile[Data]) Read(fn func(data *Data))
    func (p *JSONFile[Data]) Write(fn func(*Data) error) error
```

There is a bit more thought put into the few lines of code in this repository than you might expect.
If you want more details, see
[the blog post](https://crawshaw.io/blog/jsonfile).