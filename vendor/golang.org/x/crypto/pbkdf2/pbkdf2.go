// Code generated from golang.org/x/crypto/pbkdf2.
// Minimal implementation vendored to satisfy offline builds.

package pbkdf2

import (
	"crypto/hmac"
	"hash"
)

// Key derives a key from the password, salt and iteration count, returning a
// []byte of length keyLen. The supplied hash function is used to generate HMACs.
func Key(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	prf := hmac.New(h, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	dk := make([]byte, 0, numBlocks*hashLen)
	T := make([]byte, hashLen)
	U := make([]byte, hashLen)
	var buf [4]byte

	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf[:])
		U = prf.Sum(U[:0])
		copy(T, U)

		for i := 1; i < iter; i++ {
			prf.Reset()
			prf.Write(U)
			U = prf.Sum(U[:0])
			for j := 0; j < hashLen; j++ {
				T[j] ^= U[j]
			}
		}

		dk = append(dk, T...)
	}

	return dk[:keyLen]
}
