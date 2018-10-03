package kraken

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/fho/cryptotax/math"
	"github.com/fho/cryptotax/transaction"
)

const ExchangeName = "Kraken"

type Import struct{}

var currencies = map[string]transaction.Currency{
	"BCH":  transaction.BCH,
	"DASH": transaction.DASH,
	"EOS":  transaction.EOS,
	"EUR":  transaction.EUR,
	"XETH": transaction.ETH,
	"XLTC": transaction.LTC,
	"XXBT": transaction.BTC,
	"XBT":  transaction.BTC,
	"XXLM": transaction.XLM,
	"XXMR": transaction.XMR,
	"XXRP": transaction.XRP,
	"XZEC": transaction.ZEC,
	"ZEUR": transaction.EUR,
	"XNMC": transaction.NMC,
}

func parseCurrency(v string) (from transaction.Currency, to transaction.Currency, err error) {
	var exist bool
	var toIdx int
	//format: https://support.kraken.com/hc/en-us/articles/360001185506-Asset-Codes
	/// XBTLTC, XXLMXXBT, XXRPBCH

	str := v[:3]
	from, exist = currencies[str]
	if !exist {
		str = v[:4]
		from, exist = currencies[str]
		if !exist {
			return 0, 0, errors.New("unknown from currency pair")
		}
		toIdx = 4
	} else {
		toIdx = 3
	}

	str = v[toIdx:]
	to, exist = currencies[str]
	if !exist {
		return 0, 0, errors.New("unknown to currency pair")
	}

	return
}

func (p *Import) FromCSV(path string) ([]*transaction.Tx, error) {
	const recFields = 8
	var results []*transaction.Tx

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	csvReader := csv.NewReader(f)

	// skip first line, containing header
	_, err = csvReader.Read()
	if err != nil {
		return nil, err
	}

	// Format: "txid","ordertxid","pair","time","type","ordertype","price","cost","fee","vol","margin","misc","ledgers"
	for {
		rec, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		id := rec[0]

		currency, paycurrency, err := parseCurrency(rec[2])
		if err != nil {
			return nil, fmt.Errorf("parsing %q failed: %s", rec[2], err)
		}

		ts, err := time.Parse("2006-01-02 15:04:05.0000", rec[3])
		if err != nil {
			ts, err = time.Parse("2006-01-02 15:04:05.000", rec[3])
			if err != nil {
				return nil, fmt.Errorf("parsing %q failed: %s", rec[3], err)
			}
		}

		txType, err := transaction.NewType(rec[4])
		if err != nil {
			return nil, fmt.Errorf("parsing %q failed: %s", rec[4], err)
		}

		spotPrice, success := math.NewFloat().SetString(rec[6])
		if !success {
			log.Fatalf("import-kraken: converting %q to big float failed", rec[6])
		}

		var fee = math.NewFloat()
		if paycurrency != transaction.EUR {
			/* kraken fees are in the paycurrency, we can't handle
			* them correct when calculating the fees because we
			* don't know how much the currency was worth in eur when the
			* fees were paid */
			log.Printf("import-kraken: WARN: currency was not bought in euro, fees are ignored: %+v\n", rec)
		} else {
			fee, success = math.NewFloat().SetString(rec[8])
			if !success {
				log.Fatalf("import-kraken: converting %q to big float failed", rec[8])
			}
		}

		quantity, success := math.NewFloat().SetString(rec[9])
		if !success {
			log.Fatalf("import-kraken: converting %q to big float failed", rec[9])
		}

		txRec := transaction.Tx{
			ID:          id,
			Exchange:    ExchangeName,
			Timestamp:   ts,
			Type:        txType,
			PayCurrency: paycurrency,
			Currency:    currency,
			Quantity:    quantity,
			SpotPrice:   spotPrice,
			Fees:        fee,
		}

		results = append(results, &txRec)
	}

	return results, nil
}
