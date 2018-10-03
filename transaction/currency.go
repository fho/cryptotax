package transaction

import (
	"errors"
	"strings"
)

type Currency int

const (
	CurrencyUndef Currency = iota
	BCH
	BTC
	DASH
	EOS
	ETH
	EUR
	LTC
	NMC
	XLM
	XMR
	XRP
	ZEC
)

var strToCurrency = map[string]Currency{
	"BCH":  BCH,
	"BTC":  BTC,
	"DASH": DASH,
	"EOS":  EOS,
	"ETH":  ETH,
	"EUR":  EUR,
	"LTC":  LTC,
	"NMC":  NMC,
	"XLM":  XLM,
	"XMR":  XMR,
	"XRP":  XRP,
	"ZEC":  ZEC,
}

var currencyToStr = map[Currency]string{
	BCH:  "BCH",
	BTC:  "BTC",
	DASH: "DASH",
	EOS:  "EOS",
	ETH:  "ETH",
	EUR:  "EUR",
	LTC:  "LTC",
	NMC:  "NMC",
	XLM:  "XLM",
	XMR:  "XMR",
	XRP:  "XRP",
	ZEC:  "ZEC",
}

var ErrUndefinedCurrency = errors.New("unsupported currency")

func NewCurrency(currency string) (Currency, error) {
	res, ok := strToCurrency[strings.ToUpper(currency)]
	if !ok {
		return CurrencyUndef, ErrUndefinedCurrency
	}

	return res, nil
}

func (c Currency) String() string {
	res, ok := currencyToStr[c]
	if !ok {
		return "undefined"
	}

	return res
}
