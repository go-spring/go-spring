/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// This example demonstrates two capabilities together:
//
//  1. The Vault remote configuration provider and its secret -> bean
//     hot-reload link. app.properties imports a config document from a Vault
//     KV secret via spring.app.imports=optional:vault:.../secret/gs-config-demo.
//     A bean binds demo.message to a gs.Dync[string] field; when the example
//     writes a new value to Vault, the provider's polling watcher triggers a
//     property refresh and the bound field updates without a restart.
//
//  2. Property-level decryption (Jasypt ENC(...) equivalent). The imported
//     document also carries demo.password=ENC(<ciphertext>), which the conf
//     binding pipeline decrypts with the AES key from GS_CONFIG_DECRYPT_KEY
//     before binding, so the bean only ever sees the plaintext.
//
// The Vault client below is built directly from the SDK rather than injected,
// keeping the demonstration focused on the provider, decryption, and refresh
// link.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/hashicorp/vault/api"
	"go-spring.org/log"
	"go-spring.org/spring/conf/decrypt"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-config-vault"
)

const (
	vaultAddr  = "http://127.0.0.1:8200"
	vaultToken = "root" // dev-mode root token, see check.sh
	mount      = "secret"
	secretPath = "gs-config-demo"
	docField   = "application.properties"

	// aesKey is the base64 of the 16-byte key "1234567890123456". A real
	// deployment supplies the key out of band (a mounted Secret, a Vault Agent
	// sink); it is inlined here only to keep the smoke test self-contained.
	aesKey = "MTIzNDU2Nzg5MDEyMzQ1Ng=="
)

// Demo binds a dynamic configuration field sourced from the Vault secret and a
// decrypted secret field. It is registered as a root object so the container
// creates it eagerly.
type Demo struct {
	Message  gs.Dync[string] `value:"${demo.message:=none}"`
	Password gs.Dync[string] `value:"${demo.password:=none}"`
}

func main() {
	demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(demoBean.Interface().(*Demo))
	}()

	gs.Run()
}

func runTest(d *Demo) {
	ctx := context.Background()

	// Encrypt a password under the configured AES key, then publish a config
	// document to Vault containing both a plain and an encrypted property.
	const wantPassword = "topsecret"
	enc, err := decrypt.Encrypt(wantPassword)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "encrypt failed: %v", err)
		os.Exit(1)
	}
	wantMessage := "hello-" + time.Now().Format("150405")
	doc := fmt.Sprintf("demo.message=%s\ndemo.password=ENC(%s)\n", wantMessage, enc)

	if err := publish(ctx, doc); err != nil {
		log.Errorf(ctx, log.TagAppDef, "publish config failed: %v", err)
		os.Exit(1)
	}

	// The provider's polling watcher triggers a property refresh, which
	// re-fetches the secret, decrypts the ENC value, and updates the bound
	// gs.Dync fields. Poll until both are visible or time out.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if d.Message.Value() == wantMessage && d.Password.Value() == wantPassword {
			fmt.Println("hot-reload observed:", d.Message.Value())
			fmt.Println("decrypted password:", d.Password.Value())
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Errorf(ctx, log.TagAppDef, "timeout: message=%q (want %q) password=%q (want %q)",
		d.Message.Value(), wantMessage, d.Password.Value(), wantPassword)
	os.Exit(1)
}

// publish writes the config document to the Vault KV v2 secret.
func publish(ctx context.Context, doc string) error {
	cfg := api.DefaultConfig()
	cfg.Address = vaultAddr
	cli, err := api.NewClient(cfg)
	if err != nil {
		return err
	}
	cli.SetToken(vaultToken)
	_, err = cli.KVv2(mount).Put(ctx, secretPath, map[string]any{docField: doc})
	return err
}

// init sets the AES decrypt key and Vault token in the environment (so both the
// provider and the decryption seam are configured), and points the working
// directory at this source file's directory so the relative config path
// resolves.
// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
