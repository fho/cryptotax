package coinbase

import (
	"encoding/csv"
	"io"
	"log"
	"os"
	"time"

	"github.com/twinj/uuid"

	"github.com/fho/cryptotax/math"
	"github.com/fho/cryptotax/transaction"
)

const ExchangeName = "Coinbase"

type Import struct{}

func (p *Import) FromCSV(path string) ([]*transaction.Tx, error) {
	const recFields = 8
	var results []*transaction.Tx

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	csvReader := csv.NewReader(f)

	for {
		/* csv format:
		Timestamp,Transaction Type,Asset,Quantity Transacted,EUR Spot Price at Transaction,EUR quantity Transacted (Inclusive of Coinbase Fees),Address,Notes
		*/
		rec, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if len(rec) != recFields {
			log.Printf("import-coinbase: skipping line: %v", rec)
			continue
		}

		ts, err := time.Parse("01/02/2006", rec[0])
		if err != nil {
			log.Printf("import-coinbase: skipping line: %v", rec)
			continue
		}

		if rec[1] == "Send" || rec[1] == "Receive" {
			log.Printf("import coinbase: skipping line: %v", rec)
			continue
		}

		txType, err := transaction.NewType(rec[1])
		if err != nil {
			log.Fatalf("import-coinbase: parsing %q failed: %s", rec[1], err)
		}

		txCur, err := transaction.NewCurrency(rec[2])
		if err != nil {
			log.Fatalf("import-coinbase: parsing %q failed: %s", rec[2], err)
		}

		quantity, success := math.NewFloat().SetString(rec[3])
		if !success {
			log.Fatalf("import-coinbase: converting %q to big float failed", rec[3])
		}

		spotPrice, success := math.NewFloat().SetString(rec[4])
		if !success {
			log.Fatalf("import-coinbase: converting %q to big float failed", rec[4])
		}

		totalPriceWFees, success := math.NewFloat().SetString(rec[5])
		if !success {
			log.Fatalf("import-coinbase: converting %q to big float failed", rec[5])
		}

		var totalPrice = math.NewFloat()
		var fees = math.NewFloat()
		totalPrice.Mul(spotPrice, quantity)

		if txType == transaction.Buy {
			fees.Sub(totalPriceWFees, totalPrice)
		} else if txType == transaction.Sell {
			fees.Sub(totalPrice, totalPriceWFees)
		}

		txRec := transaction.Tx{
			ID:          uuid.NewV4().String(),
			Exchange:    ExchangeName,
			Timestamp:   ts,
			Type:        txType,
			PayCurrency: transaction.EUR,
			Currency:    txCur,
			Quantity:    quantity,
			SpotPrice:   spotPrice,
			Fees:        fees,
		}

		results = append(results, &txRec)
	}

	return results, nil
}
