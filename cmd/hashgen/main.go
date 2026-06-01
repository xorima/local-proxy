package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"local-proxy/internal/domains/auth/adapters"
)

func main() {
	user := flag.String("u", "", "Username")
	domain := flag.String("d", "", "Domain (optional)")
	flag.Parse()

	if *user == "" {
		fmt.Fprintln(os.Stderr, "username is required (-u flag)")
		os.Exit(1)
	}

	fmt.Fprint(os.Stderr, "Password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read password: %v\n", err)
		os.Exit(1)
	}
	password := string(pw)

	ntHash := adapters.NTHash(password)
	ntowfv2 := adapters.NTOWFv2(password, *user, *domain)

	fmt.Println("# Password hashes for", *user+"@"+*domain)
	fmt.Println("auth:")
	fmt.Println("  username:", *user)
	if *domain != "" {
		fmt.Println("  domain:", *domain)
	}
	fmt.Println("  pass_nt:", strings.ToUpper(hex.EncodeToString(ntHash)))
	fmt.Println("  pass_ntlmv2:", strings.ToUpper(hex.EncodeToString(ntowfv2)))
}
