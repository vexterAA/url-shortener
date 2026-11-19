package links

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/vexterAA/url-shortener/internal/core/entities"
)

const codeBase = uint64(len(entities.CodeAlphabet))

var codeSpaceSize = new(big.Int).Exp(big.NewInt(int64(codeBase)), big.NewInt(entities.CodeLength), nil)

func generateCode() (string, error) {
	// каждому числу в диапазоне соответствует ровно одна строка из 10 символов алфавита
	number, err := rand.Int(rand.Reader, codeSpaceSize)
	if err != nil {
		return "", fmt.Errorf("generate short code: %w", err)
	}

	value := number.Uint64()
	var code [entities.CodeLength]byte
	// заполняем справа налево все разряды, включая ведущие нулевые
	for i := len(code) - 1; i >= 0; i-- {
		code[i] = entities.CodeAlphabet[value%codeBase]
		value /= codeBase
	}
	return string(code[:]), nil
}
