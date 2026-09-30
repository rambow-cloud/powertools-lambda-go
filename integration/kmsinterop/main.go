// Command kmsinterop emits real Go Encryption SDK messages for Node acceptance.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"

	maskkms "github.com/rambow-cloud/powertools-lambda-go/datamasking/kms"
	"github.com/rambow-cloud/powertools-lambda-go/integration/internal/kmsfixture"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var cases []struct {
		Name       string            `json:"name"`
		Keys       []string          `json:"keys"`
		Context    map[string]string `json:"context"`
		Text       string            `json:"text"`
		Expected   string            `json:"expected"`
		Ciphertext string            `json:"ciphertext"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&cases); err != nil {
		return err
	}
	server := httptest.NewServer(&kmsfixture.Fixture{})
	defer server.Close()
	for index := range cases {
		item := &cases[index]
		provider, err := maskkms.New(context.Background(), maskkms.Config{Keys: item.Keys, ClientProvider: kmsfixture.Clients(server.URL)})
		if err != nil {
			return err
		}
		item.Ciphertext, err = provider.Encrypt(context.Background(), item.Text, item.Context)
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(cases)
}
