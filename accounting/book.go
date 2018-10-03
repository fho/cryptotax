package accounting

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/fho/cryptotax/math"
	"github.com/fho/cryptotax/transaction"
)

const TaxFreeAfter = time.Hour * 24 * 365

const TimeFormat = "02.01.2006"

type TaxRecord struct {
	Currency            transaction.Currency
	BuyTs               time.Time
	SellTs              time.Time
	SellPrice           *big.Float
	BuyPrice            *big.Float
	AdvertisingCosts    *big.Float //Werbungskosten
	HoldLongerThenAYear bool
	TaxYear             int
}

type Book struct {
	records []*credit
	buys    []*transaction.Tx
	sells   []*transaction.Tx
	taxYear int
}

type credit struct {
	balance *big.Float // remaining
	buyTx   *transaction.Tx
	sells   []*sell
}

type sell struct {
	profit                 *big.Float
	quantity               *big.Float
	holdTime               time.Duration
	tx                     *transaction.Tx
	paidWithCryptocurrency bool
}

func (s *sell) HoldTimeIsLessThenYear() bool {
	return s.holdTime <= TaxFreeAfter
}

func (s *sell) String() string {
	return fmt.Sprintf("%s %s%s @ %s for %f, taxed: %v, profit: %s",
		s.tx.Timestamp.Format(time.RFC3339), s.quantity.String(), s.tx.Currency,
		s.tx.Exchange, s.tx.PriceNoFees(), s.HoldTimeIsLessThenYear(),
		s.profit.String())
}

func (c *credit) String() string {
	res := fmt.Sprintf("%s\n  Balance: %s\n", c.buyTx, c.balance.String())

	if len(c.sells) > 0 {
		res += "  Sells:\n"
	}

	for i, s := range c.sells {
		res += "    " + s.String()
		if i+1 < len(c.sells) {
			res += "\n"
		}
	}

	return res
}

func NewBook(records []*transaction.Tx, taxYear int) (*Book, error) {
	b := Book{
		taxYear: taxYear,
	}

	for _, rec := range records {
		if rec.Type == transaction.Buy {
			b.buys = append(b.buys, rec)
			continue
		}

		if rec.Type == transaction.Sell {
			b.sells = append(b.sells, rec)
			continue
		}

		return nil, fmt.Errorf("unsupported transaction type: %s", rec.Type)
	}

	sort.Slice(b.buys, func(i, j int) bool {
		return b.buys[i].Timestamp.UnixNano() < b.buys[j].Timestamp.UnixNano()
	})

	sort.Slice(b.sells, func(i, j int) bool {
		return b.sells[i].Timestamp.UnixNano() < b.sells[j].Timestamp.UnixNano()
	})

	return &b, nil
}

func (b *Book) addCredit(quantity *big.Float, rec *transaction.Tx) (*big.Float, error) {
	if rec.PayCurrency == transaction.EUR {
		cr := credit{
			balance: math.NewFloat().Copy(quantity),
			buyTx:   rec,
		}
		log.Printf("accounting: recording buy: %+v\n", rec)

		b.records = append(b.records, &cr)
		return math.NewFloat(), nil
	}

	credit, err := b.findCredit(rec.PayCurrency, rec.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("could not find credit for buying %v", rec)
	}

	if credit.balance.Cmp(rec.PriceNoFees()) >= 0 {
		credit.balance.Sub(credit.balance, rec.PriceNoFees())

		sellRec := sell{
			tx:                     rec,
			holdTime:               rec.Timestamp.Sub(credit.buyTx.Timestamp),
			quantity:               math.NewFloat().Set(rec.PriceNoFees()),
			profit:                 math.NewFloat(),
			paidWithCryptocurrency: true,
		}

		credit.sells = append(credit.sells, &sellRec)
		log.Printf("accounting: recording trade: %s\n", sellRec.String())

		return math.NewFloat(), nil
	}

	remainingInPrice := math.NewFloat().Sub(rec.PriceNoFees(), credit.balance)
	remaining := math.NewFloat().Quo(remainingInPrice, rec.SpotPrice)

	sellRec := sell{
		tx:                     rec,
		holdTime:               rec.Timestamp.Sub(credit.buyTx.Timestamp),
		quantity:               math.NewFloat().Set(credit.balance),
		profit:                 math.NewFloat(),
		paidWithCryptocurrency: true,
	}

	credit.balance.Sub(credit.balance, sellRec.quantity)
	fmt.Println(credit.balance)
	credit.sells = append(credit.sells, &sellRec)

	return b.addCredit(remaining, rec)
}

func (b *Book) addCredits(recs []*transaction.Tx) error {
	for _, rec := range b.buys {
		_, err := b.addCredit(rec.Quantity, rec)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Book) sell(quantity *big.Float, tx *transaction.Tx) (*big.Float, error) {
	var sellRec sell
	var remaining = math.NewFloat()

	sellRec.tx = tx

	creditRec, err := b.findCredit(tx.Currency, tx.Timestamp)
	if err != nil {
		log.Printf("accounting: WARN: could not find buy record for %v: %s, assuming 100%% earning", tx, err)
		sellRec.holdTime = time.Duration(0)
		sellRec.quantity = math.NewFloat().Set(quantity)
		sellRec.profit = tx.PriceNoFees()
		return math.NewFloat(), nil
	}

	sellRec.holdTime = tx.Timestamp.Sub(creditRec.buyTx.Timestamp)

	// transaction is smaller or same then balance, we can sub everything and end it
	if creditRec.balance.Cmp(quantity) >= 0 {
		sellRec.quantity = math.NewFloat().Set(quantity)
		sellRec.profit = calcProfit(sellRec.quantity, creditRec.buyTx, tx)

		creditRec.balance.Sub(creditRec.balance, sellRec.quantity)
		creditRec.sells = append(creditRec.sells, &sellRec)

		return remaining, nil
	}

	// transaction is bigger then balance
	remaining.Sub(quantity, creditRec.balance)
	sellRec.quantity = math.NewFloat().Set(creditRec.balance)
	sellRec.profit = calcProfit(sellRec.quantity, creditRec.buyTx, tx)

	creditRec.balance.Sub(creditRec.balance, sellRec.quantity)
	creditRec.sells = append(creditRec.sells, &sellRec)

	return b.sell(remaining, tx)
}

// returns the first buy record (by timestamp) with the same currency and
// balance>0
func (b *Book) findCredit(currency transaction.Currency, from time.Time) (*credit, error) {
	var zero = math.NewFloat()

	for _, brec := range b.records {
		if brec.balance.Cmp(zero) == 0 {
			continue
		}

		if brec.buyTx.Currency != currency {
			continue
		}

		if brec.buyTx.Timestamp.After(from) {
			continue
		}

		return brec, nil
	}

	return nil, errors.New("does not exist")
}

func calcProfit(amount *big.Float, buy *transaction.Tx, sell *transaction.Tx) *big.Float {
	var res big.Float

	buyPrice := (&big.Float{}).Mul(amount, buy.SpotPrice)
	sellPrice := (&big.Float{}).Mul(amount, sell.SpotPrice)

	return res.Sub(sellPrice, buyPrice)
}

func (b *Book) Calculate() error {
	err := b.addCredits(b.buys)
	if err != nil {
		return err
	}

	for _, tx := range b.sells {
		_, err := b.sell(tx.Quantity, tx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Book) String() string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 4, 4, ' ', 0)

	tw.Write([]byte("Balance\tType\tTimestamp\tExchange\tQuantity\tSpot Price\tExchange TX ID\tTX Fees\tProfit\tHold Time in days\tTaxable\n"))
	var result string
	for _, rec := range b.records {
		result += fmt.Sprintf("%s\n", rec)
		tw.Write([]byte(fmt.Sprintf("%f %s\tBUY\t%s\t%s\t%f %s\t%f %s\t%s\t%f %s\t-\t-\t-\n",
			rec.balance, rec.buyTx.Currency,
			rec.buyTx.Timestamp.Format(time.RFC822Z),
			rec.buyTx.Exchange,
			rec.buyTx.Quantity, rec.buyTx.Currency,
			rec.buyTx.SpotPrice, rec.buyTx.PayCurrency,
			rec.buyTx.ID,
			rec.buyTx.Fees, rec.buyTx.PayCurrency)))

		for _, sell := range rec.sells {
			var sellType = "SELL"
			if sell.paidWithCryptocurrency {
				sellType = "TRADE"
			}

			tw.Write([]byte(fmt.Sprintf("-\t%s\t%s\t%s\t%f %s\t%f %s\t%s\t%f %s\t%f %s\t%f\t%v\n",
				sellType,
				sell.tx.Timestamp.Format(time.RFC822Z),
				sell.tx.Exchange,
				sell.quantity, sell.tx.Currency,
				sell.tx.SpotPrice, sell.tx.PayCurrency,
				sell.tx.ID,
				sell.tx.Fees, sell.tx.PayCurrency,
				sell.profit, sell.tx.PayCurrency,
				sell.holdTime.Hours()/24,
				sell.HoldTimeIsLessThenYear(),
			)))
		}
	}

	tw.Flush()

	return buf.String()
}

func (b *Book) TaxReport(full bool) string {
	var buf bytes.Buffer
	var count int
	var earnings = math.NewFloat()
	var loss = math.NewFloat()

	tw := tabwriter.NewWriter(&buf, 0, 4, 4, ' ', 0)
	tw.Write([]byte("# Tax Year\tHold >=1Year\tCurrency\tBuy Date\tSell Date\tSell Price\t Buy Price\tAdvertisment Costs\n"))

	for _, tr := range b.TaxRecords() {
		if !full && b.taxYear != tr.TaxYear {
			continue
		}

		if !full && tr.HoldLongerThenAYear {
			continue
		}

		count++
		tw.Write([]byte(fmt.Sprintf("%d\t%v\t%s\t%s\t%s\t%f€\t%f€\t%f€\n",
			tr.TaxYear,
			tr.HoldLongerThenAYear,
			tr.Currency,
			tr.BuyTs.Format(TimeFormat),
			tr.SellTs.Format(TimeFormat),
			tr.SellPrice,
			tr.BuyPrice,
			tr.AdvertisingCosts,
		)))

		profit := math.NewFloat().Sub(tr.SellPrice, tr.BuyPrice)
		profit = profit.Sub(profit, tr.AdvertisingCosts)

		if profit.Cmp(new(big.Float)) >= 0 {
			earnings.Add(earnings, profit)
		} else {
			loss.Add(loss, profit)
		}
	}

	tw.Flush()
	buf.Write([]byte(fmt.Sprintf("---\nCount: %d\n", count)))
	buf.Write([]byte(fmt.Sprintf("Earning: %f€\n", earnings)))
	buf.Write([]byte(fmt.Sprintf("Loss: %f€\n", loss)))

	return buf.String()
}

func (b *Book) TaxRecords() []*TaxRecord {
	var result []*TaxRecord

	includedFees := map[string]interface{}{}

	for _, rec := range b.records {
		for _, sell := range rec.sells {
			fees := math.NewFloat()
			if _, exist := includedFees[sell.tx.ID]; !exist {
				includedFees[sell.tx.ID] = struct{}{}
				fees = fees.Add(fees, sell.tx.Fees)
			}

			if _, exist := includedFees[rec.buyTx.ID]; !exist {
				includedFees[rec.buyTx.ID] = struct{}{}
				fees = fees.Add(fees, rec.buyTx.Fees)
			}

			tr := TaxRecord{
				Currency:            rec.buyTx.Currency,
				BuyTs:               rec.buyTx.Timestamp,
				SellTs:              sell.tx.Timestamp,
				SellPrice:           math.NewFloat().Mul(sell.quantity, sell.tx.SpotPrice),
				BuyPrice:            math.NewFloat().Mul(sell.quantity, rec.buyTx.SpotPrice),
				AdvertisingCosts:    fees,
				HoldLongerThenAYear: !sell.HoldTimeIsLessThenYear(),
				TaxYear:             sell.tx.Timestamp.Year(),
			}

			result = append(result, &tr)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TaxYear != result[j].TaxYear {
			return result[i].TaxYear < result[j].TaxYear
		}

		if result[i].SellTs != result[j].SellTs {
			return result[i].SellTs.Before(result[j].SellTs)
		}

		return result[i].Currency < result[j].Currency

	})

	return result
}
