package passcode

import (
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	mrand "math/rand"
	"strconv"
)

func GenSecureRandomNumber() int {

	max := new(big.Int).SetInt64(900000)
	offset := new(big.Int).SetInt64(100000)

	num, err := rand.Int(rand.Reader, max)
	if err == nil {
		passcode := num.Add(num, offset).Int64()
		return int(passcode)
	} else {
		fallbackPasscode := mrand.Intn(900000) + 100000
		return fallbackPasscode
	}

}

func HashPasscode() (int, [32]byte) {

	passcode := GenSecureRandomNumber()
	numStr := strconv.Itoa(passcode)
	data := []byte(numStr)
	
	return passcode, sha256.Sum256(data)

}
