package deploy

import "github.com/tinywasm/keyring"

type Deploy struct {
	Keys *keyring.KeyManager
}

func New() *Deploy {
	return &Deploy{
		Keys: keyring.New(),
	}
}
