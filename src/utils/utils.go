package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"
)

type CheckSumAlgorithm string

const (
	MD5    CheckSumAlgorithm = "md5"
	SHA1   CheckSumAlgorithm = "sha1"
	SHA256 CheckSumAlgorithm = "sha256"
)

func GetCheckSum(algo string, filepath string) (string, error) {
	file, err := os.Open(filepath)

	if err != nil {
		panic(err)
	}

	defer file.Close()

	var algoHash = map[string]hash.Hash{
		string(MD5):    md5.New(),
		string(SHA1):   sha1.New(),
		string(SHA256): sha256.New(),
	}

	value, ok := algoHash[algo]

	if ok {
		hash := value
		_, err = io.Copy(hash, file)

		if err != nil {
			panic(err)
		}
		checkSum := hash.Sum(nil)
		return hex.EncodeToString(checkSum[:]), nil
	} else {
		err := errors.New("unknown algo passed" + algo)
		return "", err
	}

}
