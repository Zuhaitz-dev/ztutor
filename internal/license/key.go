package license

import "crypto/ed25519"

var publicKey ed25519.PublicKey

func SetPublicKey(key ed25519.PublicKey) {
	publicKey = key
}
