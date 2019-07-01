package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fho/cryptotax/accounting"
	"github.com/fho/cryptotax/import/coinbase"
	"github.com/fho/cryptotax/import/kraken"
	"github.com/fho/cryptotax/transaction"
)

func errCheck(err error) {
	if err == nil {
		return
	}

	log.Fatalln(err)
}

/*
	TODO:
	- if a currency is bought in a cryptocurrency, the owned amount of it has to be reduced
	- add testcases
	- review big float use
*/

func main() {
	var coinbaseFileFlag string
	var krakenFileFlag string
	var taxYear uint

	flag.StringVar(&coinbaseFileFlag, "coinbase-csv", "", "path to a coinbase taxhistory csv file")
	flag.StringVar(&krakenFileFlag, "kraken-csv", "", "path to a kraken trades csv file")
	flag.UintVar(&taxYear, "tax-year", uint(time.Now().Year())-1, "year for that the report is created")
	flag.Parse()

	if len(coinbaseFileFlag) == 0 && len(krakenFileFlag) == 0 {
		fmt.Printf("Error: You have to specify the path to at least 1 csv file\n\n")
		flag.Usage()
		os.Exit(1)
	}

	var records []*transaction.Tx

	if len(coinbaseFileFlag) != 0 {
		log.Printf("reading %s", coinbaseFileFlag)
		cp := coinbase.Import{}
		coinbaseRecords, err := cp.FromCSV(coinbaseFileFlag)
		errCheck(err)
		records = append(records, coinbaseRecords...)
	}

	if len(krakenFileFlag) != 0 {
		log.Printf("reading %s", krakenFileFlag)
		cp := kraken.Import{}
		krakenRecords, err := cp.FromCSV(krakenFileFlag)
		errCheck(err)
		records = append(records, krakenRecords...)
	}

	book, err := accounting.NewBook(records, int(taxYear))
	errCheck(err)

	err = book.Calculate()
	errCheck(err)

	fmt.Println(book)
	fmt.Println()
	fmt.Println("TAX REPORT Full")
	fmt.Println(book.TaxReport(true))
	fmt.Println("================")
	fmt.Printf("TAX REPORT %v\n", taxYear)
	fmt.Println(book.TaxReport(false))

	fmt.Println()
}
