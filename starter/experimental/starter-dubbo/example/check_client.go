//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"dubbo.apache.org/dubbo-go/v3/client"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
	greet "go-spring.org/starter-dubbo/example/idl/proto"
)

// check_client is a standalone client script for manually verifying the
// starter-dubbo example server. Run it while the server is up in manual mode:
//
//	Terminal 1: go run . -manual
//	Terminal 2: go run check_client.go
func main() {
	cli, err := client.NewClient(client.WithClientURL("127.0.0.1:20000"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client: %v\n", err)
		os.Exit(1)
	}

	svc, err := greet.NewGreetService(cli)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create greet service: %v\n", err)
		os.Exit(1)
	}

	resp, err := svc.Greet(context.Background(), &greet.GreetRequest{Name: "Hello, Dubbo-Go!"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Greet: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp.Greeting)
	if resp.Greeting != "Hello, Dubbo-Go!" {
		fmt.Fprintf(os.Stderr, "unexpected greet body: %q\n", resp.Greeting)
		os.Exit(1)
	}
	fmt.Println("OK: Dubbo RPC verified")
}
