package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"aigate/internal/license"
	"aigate/internal/modules"
)

func main() {
	genKeys := flag.Bool("gen-keys", false, "generate ed25519 key pair")
	sign := flag.Bool("sign", false, "sign a license")
	privB64 := flag.String("priv", "", "private key base64")
	customer := flag.String("customer", "demo", "customer name")
	expires := flag.String("expires", "", "RFC3339 expiry, empty=never")
	out := flag.String("out", "license.json", "output path")
	maxUsers := flag.Int("max-users", 50, "max users")
	flag.Parse()

	if *genKeys {
		pub, priv, err := license.GenerateKeyPair()
		if err != nil {
			fatal(err)
		}
		fmt.Println("PUBLIC_KEY_B64 (put in configs license.public_key_b64 / embed in release):")
		fmt.Println(pub)
		fmt.Println()
		fmt.Println("PRIVATE_KEY_B64 (KEEP SECRET — vendor only, never ship to customers):")
		fmt.Println(priv)
		return
	}
	if *sign {
		if *privB64 == "" {
			fatal(fmt.Errorf("-priv required"))
		}
		priv, err := license.ParsePrivateKey(*privB64)
		if err != nil {
			fatal(err)
		}
		p := license.Payload{
			Customer:  *customer,
			ExpiresAt: *expires,
			MaxUsers:  *maxUsers,
			IssuedAt:  time.Now().UTC().Format(time.RFC3339),
			Modules: map[string]bool{
				modules.Marketplace: true, modules.Wallet: true, modules.Playground: true,
				modules.AdminChannels: true, modules.BillingOnline: true, modules.SSO: true,
				modules.Audit: true, modules.PluginRuntime: true,
				modules.Core: true, modules.Console: true,
			},
		}
		b, err := license.Sign(priv, p)
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(*out, b, 0o600); err != nil {
			fatal(err)
		}
		fmt.Println("wrote", *out)
		_ = json.NewEncoder(os.Stdout).Encode(p)
		return
	}
	fmt.Println("usage:")
	fmt.Println("  go run ./cmd/licensekit -gen-keys")
	fmt.Println("  go run ./cmd/licensekit -sign -priv <PRIVATE_B64> -customer Acme -out license.json")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
