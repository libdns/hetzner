# Vercel DNS for `libdns`

<!-- [![godoc reference](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/libdns/hetzner) -->


This package implements the libdns interfaces for the [Vercel DNS API](https://vercel.com/docs/api#endpoints/dns)

## Authenticating

To authenticate you need to supply a Vercel [APIToken](https://vercel.com/docs/api#api-basics/authentication).
For Testing purposes you can get a Testing Token from your Vercel dashboard.

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fairhat/libdns-vercel"
)

func main() {
	token := os.Getenv("LIBDNS_VERCEL_TOKEN")
	if token == "" {
		fmt.Printf("LIBDNS_VERCEL_TOKEN not set\n")
		return
	}

	zone := os.Getenv("LIBDNS_VERCEL_ZONE")
	if token == "" {
		fmt.Printf("LIBDNS_VERCEL_ZONE not set\n")
		return
	}

	p := &vercel.Provider{
		AuthAPIToken: token,
	}

	records, err := p.GetRecords(context.WithTimeout(context.Background(), time.Duration(15*time.Second)), zone)
	if err != nil {
        fmt.Printf("Error: %s", err.Error())
        return
	}

	fmt.Println(records)
}

```

