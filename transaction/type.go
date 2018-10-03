package transaction

import (
	"errors"
	"strings"
)

type Type int

const (
	TypeUndef Type = iota
	Buy
	Sell
)

var strToType = map[string]Type{
	"buy":  Buy,
	"sell": Sell,
}

var typeToStr = map[Type]string{
	Buy:  "buy",
	Sell: "sell",
}

var ErrUndefinedType = errors.New("unsupported transaction type")

func NewType(txtype string) (Type, error) {
	res, ok := strToType[strings.ToLower(txtype)]
	if ok {
		return res, nil
	}

	return TypeUndef, ErrUndefinedType
}

func (t Type) String() string {
	res, ok := typeToStr[t]
	if !ok {
		return "undefined"
	}

	return res
}
