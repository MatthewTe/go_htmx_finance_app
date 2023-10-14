package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"crypto/md5"
	"encoding/hex"

	_ "github.com/mattn/go-sqlite3"
)

type Transaction struct {
	UniqueId    string
	Date        time.Time
	Description string
	Debit       float32
	Credit      float32
}

func RebuildDatabase(dbPath string) (*sql.DB, error) {

	os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	createCoreSchema := `
	CREATE TABLE IF NOT EXISTS transactions (
		unique_id BLOB not null primary key,
		date TEXT not null,
		description TEXT,
		debit REAL,
		credit REAL
	);`

	_, err = db.Exec(createCoreSchema)
	if err != nil {
		log.Fatal("Error in creating the transactions table in the SQLite db:", err)
		return db, err
	}

	return db, nil

}

func LoadTestData(testCsvPath string, db *sql.DB) (transactions []Transaction, err error) {

	file, err := os.Open(testCsvPath)
	if err != nil {
		log.Fatal("Unable to load the test file:", err)
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Unable to read rows from the csv:", err)

	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal("Error in connecting to a database:", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO transactions(
		unique_id, 
		date, 
		description, 
		debit, 
		credit
		) values(?, ?, ?, ? , ?)`)
	if err != nil {
		log.Fatal("Error in constructing transactions insert query", err)
	}
	defer stmt.Close()

	for i := 0; i < len(records); i++ {

		// Get a unique MD5 Hash for all elements for each row in the csv:
		var rawDate, rawDescription, rawDebit, rawCredit string
		record := records[i]
		rawDate = record[0]
		rawDescription = record[1]
		rawDebit = record[2]
		rawCredit = record[3]

		// If the credit or debit value is a blank string we set it to 0.0:
		h := md5.New()
		io.WriteString(h, rawDate)
		io.WriteString(h, rawDescription)
		io.WriteString(h, rawDebit)
		io.WriteString(h, rawCredit)

		// The last value appended to the array of csv rows is an MD5 hash for al existing records:
		transactionHash := hex.EncodeToString((h.Sum(nil)))

		_, err = stmt.Exec(transactionHash, record[0], record[1], record[2], record[3])
		if err != nil {
			log.Fatal("Unable to insert a specific test transaction row into db: ", err)
		}

	}

	err = tx.Commit()
	if err != nil {
		log.Fatal("Unable to execute full insert query for test data:", err)
	}
	fmt.Println("Inserted all test data into db.")

	// Re-querying the inserted records from the database:
	rows, err := db.Query("SELECT * FROM transactions")
	if err != nil {
		log.Fatal("Unable to query the newly inserted rows into the transaction table:", err)
	}
	defer rows.Close()

	for rows.Next() {
		var extractedUniqueId, extractedDate, extractedDescription, extractedDebit, extractedCredit string

		err := rows.Scan(&extractedUniqueId, &extractedDate, &extractedDescription, &extractedDebit, &extractedCredit)
		if err != nil {
			log.Fatal("Error in querying row from test database:", err)
		}

		// Correctly formatting all of the data from the db:
		//fmt.Println(extractedUniqueId, extractedDate, extractedDescription, extractedDebit, extractedCredit)

		var formattedCredit float32
		if extractedCredit == "" {
			formattedCredit = 0.0
		} else {
			formattedCredit64, err := strconv.ParseFloat(extractedCredit, 32)
			formattedCredit = float32(formattedCredit64)
			if err != nil {
				log.Fatal("Error in converting the credit value to a format: ", err)
			}
		}
		var formattedDebit float32
		if extractedDebit == "" {
			formattedDebit = 0.0
		} else {
			formattedDebit64, err := strconv.ParseFloat(extractedDebit, 32)
			formattedDebit = float32(formattedDebit64)
			if err != nil {
				log.Fatal("Error in converting the debit value to a format: ", err)
			}
		}

		transactionTime, err := time.Parse("2006-01-02", extractedDate)
		if err != nil {
			log.Fatal("Error in loading transaction time into a date struct", err)
		}

		transactions = append(transactions, Transaction{
			UniqueId:    extractedUniqueId,
			Date:        transactionTime,
			Description: extractedDescription,
			Debit:       formattedDebit,
			Credit:      formattedCredit,
		})

		err = rows.Err()
		if err != nil {
			log.Fatal("Error in closing the db query connection:", err)
		}
	}

	return transactions, nil

}

func ReadAllTransactions(db *sql.DB) (transactions []Transaction, err error) {
	// Re-querying the inserted records from the database:
	rows, err := db.Query("SELECT * FROM transactions")
	if err != nil {
		log.Fatal("Unable to query the newly inserted rows into the transaction table:", err)
	}
	defer rows.Close()

	for rows.Next() {
		var extractedUniqueId, extractedDate, extractedDescription, extractedDebit, extractedCredit string

		err := rows.Scan(&extractedUniqueId, &extractedDate, &extractedDescription, &extractedDebit, &extractedCredit)
		if err != nil {
			log.Fatal("Error in querying row from test database:", err)
		}

		// Correctly formatting all of the data from the db:
		//fmt.Println(extractedUniqueId, extractedDate, extractedDescription, extractedDebit, extractedCredit)

		var formattedCredit float32
		if extractedCredit == "" {
			formattedCredit = 0.0
		} else {
			formattedCredit64, err := strconv.ParseFloat(extractedCredit, 32)
			formattedCredit = float32(formattedCredit64)
			if err != nil {
				log.Fatal("Error in converting the credit value to a format: ", err)
			}
		}
		var formattedDebit float32
		if extractedDebit == "" {
			formattedDebit = 0.0
		} else {
			formattedDebit64, err := strconv.ParseFloat(extractedDebit, 32)
			formattedDebit = float32(formattedDebit64)
			if err != nil {
				log.Fatal("Error in converting the debit value to a format: ", err)
			}
		}

		transactionTime, err := time.Parse("2006-01-02", extractedDate)
		if err != nil {
			log.Fatal("Error in loading transaction time into a date struct", err)
		}

		transactions = append(transactions, Transaction{
			UniqueId:    extractedUniqueId,
			Date:        transactionTime,
			Description: extractedDescription,
			Debit:       formattedDebit,
			Credit:      formattedCredit,
		})

		err = rows.Err()
		if err != nil {
			log.Fatal("Error in closing the db query connection:", err)
		}
	}

	return transactions, nil

}

// Database Transaction Resampling:
type Row struct {
	income   float64
	expenses float64
	balance  float64
}

type BudgetStatement struct {
	dateTimeIndex                       []time.Time
	expenseTimeseries, incomeTimeseries []float64
	dailyResample                       map[time.Time]Row
}

func (b *BudgetStatement) resampleTimeseriesDaily() {
	// Function resamples the expense and income timeseries to a daily step.

	// Step 1: Generate a full list of all dates between the date ranges in the timeseries:
	var earliestDate, lastDate time.Time = b.dateTimeIndex[len(b.dateTimeIndex)-1], b.dateTimeIndex[0]

	numDaysBetweenDates := int(lastDate.Sub(earliestDate).Hours() / 24)

	for i := 0; i <= numDaysBetweenDates; i++ {

		// Build the map from each newly generated date:
		b.dailyResample[earliestDate.AddDate(0, 0, i)] = Row{}
	}

	fmt.Printf("First Day: %s,\nLast Date: %s\n", earliestDate, lastDate)
	fmt.Printf("Number of Hours between these two dates: %d\n", numDaysBetweenDates)

	// Step 2: Iterating through the primary date time index and updating the associated date in the new map.
	for i, v := range b.dateTimeIndex {

		var oldIncomeVal, oldExpenseVal float64 = b.dailyResample[v].income, b.dailyResample[v].expenses

		b.dailyResample[v] = Row{
			income:   oldIncomeVal + b.incomeTimeseries[i],
			expenses: oldExpenseVal + b.expenseTimeseries[i]}
	}

	// Resorting the resampled index after incorporating resampling the income and expense data:
	sortedIndex := []time.Time{}

	for i := range b.dailyResample {
		sortedIndex = append(sortedIndex, i)
	}

	sort.Slice(sortedIndex, func(i, j int) bool {
		return sortedIndex[i].Before(sortedIndex[j])
	})

	// Now we rebuild the new, sorted index and calculates the balance based on income and expense:
	resortedResampleMap := map[time.Time]Row{}
	currentBalance := 0.0

	for _, v := range sortedIndex {

		changeInBalance := b.dailyResample[v].income - b.dailyResample[v].expenses
		currentBalance = currentBalance + changeInBalance

		resortedResampleMap[v] = Row{
			income:   b.dailyResample[v].income,
			expenses: b.dailyResample[v].expenses,
			balance:  currentBalance,
		}

	}

	b.dailyResample = resortedResampleMap
}

func LoadBudgetFromCSV(transactions []Transaction) (currentBudgetStatemen BudgetStatement, err error) {

	// Appending each date time string to array:
	var dateTimeIndex = []time.Time{}
	var expensesTimeseries = []float64{}
	var incomeTimeseries = []float64{}

	for i := 0; i < len(transactions); i++ {

		currentTransaction := transactions[i]

		dateTimeIndex = append(dateTimeIndex, currentTransaction.Date)
		expensesTimeseries = append(expensesTimeseries, float64(currentTransaction.Debit))
		incomeTimeseries = append(incomeTimeseries, float64(currentTransaction.Credit))

	}

	currentBudgetStatement := BudgetStatement{
		dateTimeIndex:     dateTimeIndex,
		expenseTimeseries: expensesTimeseries,
		incomeTimeseries:  incomeTimeseries,
		dailyResample:     make(map[time.Time]Row),
	}

	currentBudgetStatement.resampleTimeseriesDaily()

	return currentBudgetStatement, nil

}
