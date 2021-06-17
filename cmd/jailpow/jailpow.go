package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/redpwn/jail/pow"
)

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: jailpow challenge")
	}
	c, err := pow.DecodeChallenge(os.Args[1])
	if err != nil {
		return fmt.Errorf("decode challenge: %w", err)
	}
	fmt.Println(c.Solve())
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
