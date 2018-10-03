package transaction

import (
	"fmt"
	"math/big"
	"time"

	"github.com/fho/cryptotax/math"
)

type Tx struct {
	ID          string
	Exchange    string
	Timestamp   time.Time
	Type        Type
	PayCurrency Currency // the currency that is paid with
	Currency    Currency // the curency that is bought
	Quantity    *big.Float
	SpotPrice   *big.Float
	Fees        *big.Float
}

func (r *Tx) String() string {
	return fmt.Sprintf("%s %s %s %s @ %s for %f %s + %s€ fees",
		r.Timestamp.Format(time.RFC3339), r.Type, r.Quantity.String(), r.Currency,
		r.Exchange, r.PriceNoFees(), r.PayCurrency, r.Fees.String())
}

func (r *Tx) PriceNoFees() *big.Float {
	return math.NewFloat().Mul(r.Quantity, r.SpotPrice)
}
