package main

import(
	"context"
	"fmt"
	"github.com/krishnassh/gophertube/internal/cli"
	"os"
)

func main(){
	gophertube := cli.New()
	if err := gophertube.Run(context.Background(), os.Args);
	err != nil{
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
