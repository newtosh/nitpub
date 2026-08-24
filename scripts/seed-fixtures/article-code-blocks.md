## Handler

> [!TIP]
> Use the Preview tab while composing to confirm highlighting before you publish.

### Go service

```go
package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

### Deploy

```bash
cd web && npm run build
go build -o nitpub ./cmd/nitpub
scp nitpub user@host:/usr/local/bin/
ssh user@host systemctl restart nitpub
```
