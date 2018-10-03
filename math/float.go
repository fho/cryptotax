package math

import "math/big"

const FloatPrec = 100

func NewFloat() *big.Float {
	return new(big.Float).SetPrec(FloatPrec)
}
