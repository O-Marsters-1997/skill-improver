# Handler Example

```go
func Handle(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    fmt.Fprintln(w, "ok")
}
```

The snippet above is deliberately inside a fence, so a highlight landing in it has to
expand to the whole block rather than corrupt the code.
